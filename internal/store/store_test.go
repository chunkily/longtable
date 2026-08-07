package store

import (
	"path/filepath"
	"testing"

	"longtable/internal/db"
)

// newTestStore returns a Store backed by a fresh SQLite file in a
// per-test temp directory, with the schema already created.
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

// New runs on every startup, against a data file that usually already
// has the schema. Every statement is CREATE ... IF NOT EXISTS, so this
// is the whole of what used to be a migration suite.
func TestNew_IsIdempotent(t *testing.T) {
	s := newTestStore(t)

	if _, err := New(s.db); err != nil {
		t.Fatalf("second New: %v", err)
	}
	if _, err := New(s.db); err != nil {
		t.Fatalf("third New: %v", err)
	}
}
