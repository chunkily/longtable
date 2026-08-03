package store

import (
	"database/sql"
	"errors"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Asset is a piece of uploaded media (map background, token art).
// The actual bytes live on disk (see internal/blobstore), addressed by
// ContentHash; this row is just the metadata plus a stable ID that
// scenes/tokens reference. Because lookup is by content hash, uploading
// the same file again — even in a different room, even much later —
// reuses the existing row instead of duplicating it.
type Asset struct {
	ID          string
	ContentHash string
	Filename    string
	MimeType    string
	ByteSize    int64
	CreatedAt   string
}

func (s *Store) FindAssetByHash(hash string) (Asset, error) {
	return s.scanAsset(s.db.QueryRow(
		`SELECT id, content_hash, filename, mime_type, byte_size, created_at FROM asset WHERE content_hash = ?`, hash,
	))
}

func (s *Store) GetAsset(id string) (Asset, error) {
	return s.scanAsset(s.db.QueryRow(
		`SELECT id, content_hash, filename, mime_type, byte_size, created_at FROM asset WHERE id = ?`, id,
	))
}

func (s *Store) scanAsset(row *sql.Row) (Asset, error) {
	var a Asset
	if err := row.Scan(&a.ID, &a.ContentHash, &a.Filename, &a.MimeType, &a.ByteSize, &a.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Asset{}, ErrNotFound
		}
		return Asset{}, err
	}
	return a, nil
}

// LibraryAsset is an asset as it appears in one room's library: the
// asset itself plus the things that belong to the room's copy of it
// rather than to the file. Name and Attribution are per-room because two
// groups can legitimately call and credit the same picture differently,
// and because one room's notes aren't the other's to see.
type LibraryAsset struct {
	Asset
	Name        string
	Attribution string
	// GridSize is the map's pixels per square, measured when it was
	// aligned. Nil for anything nobody aligned — every token, and any map
	// added before the assets page existed.
	GridSize *int
	AddedAt  string
}

// AddAssetToRoom puts an asset in a room's library, or updates the
// details if it's already there. Idempotent by design: re-uploading a
// file someone already added is a normal thing to do (it's how you
// discover it was already there), and it should read as a no-op rather
// than an error.
func (s *Store) AddAssetToRoom(roomID, assetID, name, attribution string, gridSize *int) error {
	_, err := s.db.Exec(`
		INSERT INTO room_asset (room_id, asset_id, name, attribution, grid_size, added_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (room_id, asset_id) DO UPDATE SET
			-- Each field only overwrites when something was actually
			-- supplied, so a later upload that skipped the name, the credit
			-- or the grid step doesn't wipe what an earlier one recorded.
			name = CASE
				WHEN excluded.name != '' THEN excluded.name
				ELSE room_asset.name
			END,
			attribution = CASE
				WHEN excluded.attribution != '' THEN excluded.attribution
				ELSE room_asset.attribution
			END,
			grid_size = COALESCE(excluded.grid_size, room_asset.grid_size)`,
		roomID, assetID, name, attribution, gridSize, time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

// UpdateRoomAsset renames and re-credits a room's copy of an asset,
// leaving the stored image alone. Both fields are written as given,
// including empty — this is someone editing a form, so clearing the
// credit has to be possible, unlike the upload path where an absent
// field means "not supplied" rather than "make it blank".
//
// Returns ErrNotFound when the asset isn't in this room's library, which
// is also the answer for an asset that belongs to some other room: the
// caller can't tell those apart, and shouldn't be able to.
func (s *Store) UpdateRoomAsset(roomID, assetID, name, attribution string) error {
	res, err := s.db.Exec(
		`UPDATE room_asset SET name = ?, attribution = ? WHERE room_id = ? AND asset_id = ?`,
		name, attribution, roomID, assetID,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListRoomAssets returns a room's library, newest first — the order a
// picker wants, since the thing you just uploaded is the thing you're
// most likely reaching for.
func (s *Store) ListRoomAssets(roomID string) ([]LibraryAsset, error) {
	rows, err := s.db.Query(`
		SELECT a.id, a.content_hash, a.filename, a.mime_type, a.byte_size, a.created_at,
		       ra.name, ra.attribution, ra.grid_size, ra.added_at
		FROM room_asset ra
		JOIN asset a ON a.id = ra.asset_id
		WHERE ra.room_id = ?
		ORDER BY ra.added_at DESC, a.id`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LibraryAsset
	for rows.Next() {
		la, err := scanLibraryAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, la)
	}
	return out, rows.Err()
}

// GetRoomAsset returns one entry of a room's library, or ErrNotFound if
// the room doesn't have that asset.
func (s *Store) GetRoomAsset(roomID, assetID string) (LibraryAsset, error) {
	la, err := scanLibraryAsset(s.db.QueryRow(`
		SELECT a.id, a.content_hash, a.filename, a.mime_type, a.byte_size, a.created_at,
		       ra.name, ra.attribution, ra.grid_size, ra.added_at
		FROM room_asset ra
		JOIN asset a ON a.id = ra.asset_id
		WHERE ra.room_id = ? AND ra.asset_id = ?`, roomID, assetID))
	if errors.Is(err, sql.ErrNoRows) {
		return LibraryAsset{}, ErrNotFound
	}
	return la, err
}

// rowScanner is what *sql.Row and *sql.Rows have in common, so one entry
// of a library scans the same way whether it came from a lookup or a
// listing.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanLibraryAsset(row rowScanner) (LibraryAsset, error) {
	var la LibraryAsset
	if err := row.Scan(
		&la.ID, &la.ContentHash, &la.Filename, &la.MimeType, &la.ByteSize, &la.CreatedAt,
		&la.Name, &la.Attribution, &la.GridSize, &la.AddedAt,
	); err != nil {
		return LibraryAsset{}, err
	}
	// Rows added before names existed have none, and a nameless entry in
	// a grid you search by name is one you can't find. The filename is
	// what the library used to show, so it stays the fallback.
	if la.Name == "" {
		la.Name = DisplayNameFromFilename(la.Filename)
	}
	return la, nil
}

// DisplayNameFromFilename is the default name for an asset: the filename
// without its extension. The client fills the field in with this before
// uploading, so the stored name is a real editable value; this copy is
// for rows that predate the field.
func DisplayNameFromFilename(filename string) string {
	base := path.Base(strings.ReplaceAll(filename, `\`, "/"))
	if ext := path.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	if base == "" || base == "." || base == "/" {
		return "Untitled"
	}
	return base
}

// AssetInRoom reports whether an asset is in a room's library. This is
// the check that stands between a scene or token and another room's art:
// asset IDs are unguessable, but "unguessable" is not the same as
// "scoped", and one leaked ID shouldn't be usable elsewhere.
func (s *Store) AssetInRoom(roomID, assetID string) (bool, error) {
	var one int
	err := s.db.QueryRow(
		`SELECT 1 FROM room_asset WHERE room_id = ? AND asset_id = ?`, roomID, assetID,
	).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) CreateAsset(hash, filename, mimeType string, byteSize int64) (Asset, error) {
	a := Asset{
		ID:          uuid.NewString(),
		ContentHash: hash,
		Filename:    filename,
		MimeType:    mimeType,
		ByteSize:    byteSize,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	_, err := s.db.Exec(
		`INSERT INTO asset (id, content_hash, filename, mime_type, byte_size, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		a.ID, a.ContentHash, a.Filename, a.MimeType, a.ByteSize, a.CreatedAt,
	)
	if err != nil {
		return Asset{}, err
	}
	return a, nil
}
