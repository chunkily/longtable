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
