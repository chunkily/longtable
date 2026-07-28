package store

import (
	"path/filepath"
	"testing"

	"longtable/internal/db"
)

// newTestStore returns a Store backed by a fresh SQLite file in a
// per-test temp directory, with the schema already migrated.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	s, err := New(sqlDB)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return s
}
