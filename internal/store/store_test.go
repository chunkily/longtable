package store

import (
	"database/sql"
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

// Migrating a database that already has the schema must be a no-op —
// New runs on every startup, against a data file that has usually been
// migrated many times before.
func TestMigrate_IsIdempotent(t *testing.T) {
	s := newTestStore(t)

	if err := s.migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if err := s.migrate(); err != nil {
		t.Fatalf("third migrate: %v", err)
	}
}

// Databases created before drawing authorship existed don't get the new
// column from CREATE TABLE IF NOT EXISTS — dropping it back out
// simulates such a file and checks migrate adds it again.
func TestMigrate_AddsDrawingCreatorToExistingDatabase(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.db.Exec(`ALTER TABLE drawing DROP COLUMN created_by_participant_id`); err != nil {
		t.Fatalf("drop column to simulate an older database: %v", err)
	}
	if hasColumn(t, s.db, "drawing", "created_by_participant_id") {
		t.Fatal("column still present after DROP COLUMN; test setup is not simulating an older database")
	}

	if err := s.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !hasColumn(t, s.db, "drawing", "created_by_participant_id") {
		t.Fatal("migrate did not add drawing.created_by_participant_id")
	}
}

func hasColumn(t *testing.T, database *sql.DB, table, column string) bool {
	t.Helper()

	var count int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column,
	).Scan(&count); err != nil {
		t.Fatalf("inspect %s columns: %v", table, err)
	}
	return count > 0
}
