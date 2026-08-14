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
// has the schema. Every statement in createTables is CREATE ... IF NOT
// EXISTS, and addMissingColumns has to be just as repeatable — a second
// ALTER TABLE for a column that is already there is an error, not a
// no-op.
func TestNew_IsIdempotent(t *testing.T) {
	s := newTestStore(t)

	if _, err := New(s.db); err != nil {
		t.Fatalf("second New: %v", err)
	}
	if _, err := New(s.db); err != nil {
		t.Fatalf("third New: %v", err)
	}
}

// The case addMissingColumns exists for, and the one a fresh test
// database never reaches: a data file created before the column existed.
// CREATE TABLE IF NOT EXISTS does nothing to a table that is already
// there, so without the ALTER every query naming `filled` fails with "no
// such column" — which takes the drawing feature down entirely rather
// than merely starting it empty.
func TestNew_AddsAColumnToATableThatAlreadyExists(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	// The drawing table as it stood before `filled` was added.
	_, err = sqlDB.Exec(`
		CREATE TABLE drawing (
			id                        TEXT PRIMARY KEY,
			scene_id                  TEXT NOT NULL,
			kind                      TEXT NOT NULL,
			points                    TEXT NOT NULL,
			color                     TEXT NOT NULL,
			created_by_participant_id TEXT,
			created_at                TEXT NOT NULL
		);
		INSERT INTO drawing (id, scene_id, kind, points, color, created_at)
		VALUES ('old-1', 'scene-1', 'rect', '[]', '#000000', '2026-01-01T00:00:00Z');
	`)
	if err != nil {
		t.Fatalf("seed old schema: %v", err)
	}

	s, err := New(sqlDB)
	if err != nil {
		t.Fatalf("new store against old schema: %v", err)
	}

	has, err := s.hasColumn("drawing", "filled")
	if err != nil {
		t.Fatalf("hasColumn: %v", err)
	}
	if !has {
		t.Fatal("filled column was not added to the existing drawing table")
	}

	// The row that predates the column reads as unfilled rather than
	// erroring, which is what the DEFAULT 0 is for.
	var filled bool
	if err := sqlDB.QueryRow(`SELECT filled FROM drawing WHERE id = 'old-1'`).Scan(&filled); err != nil {
		t.Fatalf("read filled on a pre-existing row: %v", err)
	}
	if filled {
		t.Error("a drawing from before the column existed should read as unfilled")
	}
}

func TestHasColumn_SaysNoForOneThatIsNotThere(t *testing.T) {
	s := newTestStore(t)

	has, err := s.hasColumn("drawing", "no_such_column")
	if err != nil {
		t.Fatalf("hasColumn: %v", err)
	}
	if has {
		t.Error("hasColumn found a column that does not exist")
	}
}
