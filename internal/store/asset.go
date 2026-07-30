package store

import (
	"database/sql"
	"errors"
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
// rather than to the file. Attribution is per-room because two groups
// can legitimately credit the same picture differently, and because one
// room's notes aren't the other's to see.
type LibraryAsset struct {
	Asset
	Attribution string
	AddedAt     string
}

// AddAssetToRoom puts an asset in a room's library, or updates the
// attribution if it's already there. Idempotent by design: re-uploading
// a file someone already added is a normal thing to do (it's how you
// discover it was already there), and it should read as a no-op rather
// than an error.
func (s *Store) AddAssetToRoom(roomID, assetID, attribution string) error {
	_, err := s.db.Exec(`
		INSERT INTO room_asset (room_id, asset_id, attribution, added_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (room_id, asset_id) DO UPDATE SET
			attribution = CASE
				-- Only overwrite when something was actually supplied, so a
				-- later upload without attribution doesn't wipe the credit
				-- an earlier one recorded.
				WHEN excluded.attribution != '' THEN excluded.attribution
				ELSE room_asset.attribution
			END`,
		roomID, assetID, attribution, time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

// ListRoomAssets returns a room's library, newest first — the order a
// picker wants, since the thing you just uploaded is the thing you're
// most likely reaching for.
func (s *Store) ListRoomAssets(roomID string) ([]LibraryAsset, error) {
	rows, err := s.db.Query(`
		SELECT a.id, a.content_hash, a.filename, a.mime_type, a.byte_size, a.created_at,
		       ra.attribution, ra.added_at
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
		var la LibraryAsset
		if err := rows.Scan(
			&la.ID, &la.ContentHash, &la.Filename, &la.MimeType, &la.ByteSize, &la.CreatedAt,
			&la.Attribution, &la.AddedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, la)
	}
	return out, rows.Err()
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
