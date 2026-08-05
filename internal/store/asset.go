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

// AssetKind is what a room keeps a picture for: art to put on a token,
// or a map to put under one. Per-room like the name and the credit,
// since the same picture can be one group's boss portrait and another's
// battle map.
type AssetKind string

const (
	AssetKindToken AssetKind = "token"
	AssetKindMap   AssetKind = "map"
)

// Valid reports whether k is one of the two kinds. The empty string is
// not: callers use it as "not supplied", which is a different thing from
// a kind and must not reach the database as one.
func (k AssetKind) Valid() bool {
	return k == AssetKindToken || k == AssetKindMap
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
	// Kind sorts the library into its two tabs, and decides which pickers
	// offer this asset first. Never empty on a row that came back from the
	// database — the column defaults to 'token'.
	Kind AssetKind
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
// An empty kind means "not supplied", the same sentinel name and
// attribution use.
func (s *Store) AddAssetToRoom(roomID, assetID, name, attribution string, kind AssetKind, gridSize *int) error {
	// kind is bound three times rather than read off `excluded` like its
	// neighbours, because the row ON CONFLICT sees has already had the
	// column default applied — by then "not supplied" and "token" are the
	// same value, and an old client's silence would overwrite a map.
	_, err := s.db.Exec(`
		INSERT INTO room_asset (room_id, asset_id, name, attribution, kind, grid_size, added_at)
		VALUES (?, ?, ?, ?, COALESCE(NULLIF(?, ''), 'token'), ?, ?)
		ON CONFLICT (room_id, asset_id) DO UPDATE SET
			-- Each field only overwrites when something was actually
			-- supplied, so a later upload that skipped the name, the credit
			-- or the grid step doesn't wipe what an earlier one recorded.
			--
			-- Worth knowing what that means in practice, because the two
			-- halves behave differently through the assets page: it always
			-- sends a name, defaulting the box to the filename, so re-adding
			-- bytes already in the room *renames* the entry. It only sends a
			-- credit if someone typed one, so the credit survives. That's
			-- intended — an upload says what the image is called now — and
			-- e2e pins it in asset-library.spec.ts.
			name = CASE
				WHEN excluded.name != '' THEN excluded.name
				ELSE room_asset.name
			END,
			attribution = CASE
				WHEN excluded.attribution != '' THEN excluded.attribution
				ELSE room_asset.attribution
			END,
			kind = CASE
				WHEN ? != '' THEN ?
				ELSE room_asset.kind
			END,
			grid_size = COALESCE(excluded.grid_size, room_asset.grid_size)`,
		roomID, assetID, name, attribution, string(kind), gridSize,
		time.Now().UTC().Format(time.RFC3339Nano), string(kind), string(kind),
	)
	return err
}

// UpdateRoomAsset renames, re-credits and reclassifies a room's copy of
// an asset, leaving the stored image alone. Name and attribution are
// written as given, including an empty credit — this is someone editing
// a form, so clearing the credit has to be possible, unlike the upload
// path where an absent field means "not supplied" rather than "make it
// blank". Kind is the exception and keeps the upload path's rule, an
// empty one leaving the asset where it is: there is no third kind for
// "cleared" to mean, so silence can only be silence.
//
// Reclassifying is possible at all because the migration that first
// sorted a library into tokens and maps could only guess, and a guess
// with no way to correct it is worse than no guess.
//
// Returns ErrNotFound when the asset isn't in this room's library, which
// is also the answer for an asset that belongs to some other room: the
// caller can't tell those apart, and shouldn't be able to.
func (s *Store) UpdateRoomAsset(roomID, assetID, name, attribution string, kind AssetKind) error {
	res, err := s.db.Exec(`
		UPDATE room_asset
		SET name = ?, attribution = ?, kind = CASE WHEN ? != '' THEN ? ELSE kind END
		WHERE room_id = ? AND asset_id = ?`,
		name, attribution, string(kind), string(kind), roomID, assetID,
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

// RemoveAssetFromRoom takes an asset off one room's shelf. It deletes
// the library row and nothing else: the asset row and the file behind it
// are global and content-addressed, so any other room that added the
// same picture keeps it, and this room can get it back by adding the
// file again.
//
// It also doesn't reach onto the table. A scene or token already using
// the image goes on using it — those hold the asset ID, which is still
// good, and yanking the art out from under a scene mid-session is not
// what "remove it from the library" asks for. Erasing a file everywhere
// is a Host's job and a different feature; see the host-moderate-assets
// story.
//
// Returns ErrNotFound when the asset isn't in this room's library, which
// is also the answer for another room's asset — the same blind spot
// UpdateRoomAsset keeps, for the same reason.
func (s *Store) RemoveAssetFromRoom(roomID, assetID string) error {
	res, err := s.db.Exec(
		`DELETE FROM room_asset WHERE room_id = ? AND asset_id = ?`, roomID, assetID,
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
		       ra.name, ra.attribution, ra.kind, ra.grid_size, ra.added_at
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
		       ra.name, ra.attribution, ra.kind, ra.grid_size, ra.added_at
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
	// kind lands in a plain string first, the way Visibility does in
	// token.go — the driver has no reason to know about our named types.
	var kind string
	if err := row.Scan(
		&la.ID, &la.ContentHash, &la.Filename, &la.MimeType, &la.ByteSize, &la.CreatedAt,
		&la.Name, &la.Attribution, &kind, &la.GridSize, &la.AddedAt,
	); err != nil {
		return LibraryAsset{}, err
	}
	la.Kind = AssetKind(kind)
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
