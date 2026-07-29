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

// A database written before the circle tool became an ellipse tool
// holds 'circle' rows storing [centre, edge]. Migration has to restate
// the table's CHECK constraint — which SQLite can only do by rebuilding
// the table — and reinterpret that geometry as the corner pair an
// ellipse uses, so an old circle comes back the same size and place.
func TestMigrate_ConvertsCircleDrawingsToEllipses(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	// A line that must survive the rebuild untouched, alongside the
	// circles being converted.
	line, err := s.CreateDrawing(Drawing{SceneID: scene.ID, Kind: DrawingKindLine, Points: []Point{{X: 0, Y: 0}, {X: 1, Y: 1}}, Color: "#000000"})
	if err != nil {
		t.Fatalf("CreateDrawing: %v", err)
	}

	restoreOldDrawingSchema(t, s)

	// centre (100, 100), radius 50 — an ellipse spanning (50,50)-(150,150).
	insertLegacyCircle(t, s, scene.ID, "circle-1", `[{"x":100,"y":100},{"x":100,"y":150}]`)
	// Geometry this migration can't read is left alone rather than guessed at.
	insertLegacyCircle(t, s, scene.ID, "circle-2", `[{"x":1,"y":1}]`)

	if err := s.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	drawings, err := s.ListDrawingsForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	byID := make(map[string]Drawing, len(drawings))
	for _, d := range drawings {
		byID[d.ID] = d
	}
	if len(byID) != 3 {
		t.Fatalf("len(drawings) = %d, want 3", len(byID))
	}

	converted := byID["circle-1"]
	if converted.Kind != DrawingKindEllipse {
		t.Fatalf("Kind = %q, want ellipse", converted.Kind)
	}
	want := []Point{{X: 50, Y: 50}, {X: 150, Y: 150}}
	if len(converted.Points) != 2 || converted.Points[0] != want[0] || converted.Points[1] != want[1] {
		t.Fatalf("Points = %+v, want %+v", converted.Points, want)
	}

	if unreadable := byID["circle-2"]; unreadable.Kind != DrawingKindEllipse || len(unreadable.Points) != 1 {
		t.Fatalf("unreadable circle = %+v, want kind ellipse with its points untouched", unreadable)
	}

	if kept := byID[line.ID]; kept.Kind != DrawingKindLine || len(kept.Points) != 2 {
		t.Fatalf("line drawing = %+v, want it carried through the rebuild unchanged", kept)
	}

	// The rebuilt table must reject the kind it was rebuilt to remove,
	// and still accept the ones it allows.
	if _, err := s.db.Exec(
		`INSERT INTO drawing (id, scene_id, kind, points, color, created_at) VALUES ('x', ?, 'circle', '[]', '#000000', '2026-07-29')`,
		scene.ID,
	); err == nil {
		t.Fatal("expected the rebuilt table to reject kind 'circle'")
	}
	if _, err := s.CreateDrawing(Drawing{SceneID: scene.ID, Kind: DrawingKindEllipse, Points: []Point{{X: 0, Y: 0}, {X: 4, Y: 2}}, Color: "#000000"}); err != nil {
		t.Fatalf("CreateDrawing(ellipse) after migration: %v", err)
	}

	// Running again against the migrated database changes nothing.
	if err := s.migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	after, err := s.ListDrawingsForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	if len(after) != 4 {
		t.Fatalf("len(drawings) = %d after re-migrating, want 4", len(after))
	}
}

// restoreOldDrawingSchema puts the drawing table back the way it looked
// before ellipses existed, so the migration has something to migrate.
func restoreOldDrawingSchema(t *testing.T, s *Store) {
	t.Helper()

	if _, err := s.db.Exec(`
		CREATE TABLE drawing_old (
			id                        TEXT PRIMARY KEY,
			scene_id                  TEXT NOT NULL REFERENCES scene(id) ON DELETE CASCADE,
			kind                      TEXT NOT NULL CHECK (kind IN ('freehand', 'line', 'rect', 'circle')),
			points                    TEXT NOT NULL,
			color                     TEXT NOT NULL,
			created_by_participant_id TEXT REFERENCES participant(id) ON DELETE SET NULL,
			created_at                TEXT NOT NULL
		);
		INSERT INTO drawing_old SELECT id, scene_id, kind, points, color, created_by_participant_id, created_at FROM drawing;
		DROP TABLE drawing;
		ALTER TABLE drawing_old RENAME TO drawing;
		CREATE INDEX IF NOT EXISTS idx_drawing_scene ON drawing(scene_id);
	`); err != nil {
		t.Fatalf("restore pre-ellipse drawing schema: %v", err)
	}
}

func insertLegacyCircle(t *testing.T, s *Store, sceneID, id, pointsJSON string) {
	t.Helper()

	if _, err := s.db.Exec(
		`INSERT INTO drawing (id, scene_id, kind, points, color, created_at) VALUES (?, ?, 'circle', ?, '#cc0000', '2026-07-29T00:00:00Z')`,
		id, sceneID, pointsJSON,
	); err != nil {
		t.Fatalf("insert legacy circle: %v", err)
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
