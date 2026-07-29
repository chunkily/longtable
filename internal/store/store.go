// Package store owns Longtable's SQLite schema and the typed queries
// used by the REST API, the WebSocket hub, and the CLI admin commands.
package store

import (
	"database/sql"
	"fmt"
)

type Store struct {
	db *sql.DB
}

// New wraps db and ensures the schema exists.
func New(db *sql.DB) (*Store, error) {
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	if err := s.createTables(); err != nil {
		return err
	}
	// Columns added to a table after its first release need an explicit
	// ALTER TABLE — the CREATE TABLE statements above only take effect
	// on a database file that doesn't exist yet.
	return s.addColumnIfMissing("drawing", "created_by_participant_id",
		`TEXT REFERENCES participant(id) ON DELETE SET NULL`)
}

func (s *Store) createTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS room (
			id                TEXT PRIMARY KEY,
			slug              TEXT NOT NULL UNIQUE,
			name              TEXT NOT NULL,
			gm_password_hash  TEXT NOT NULL,
			active_scene_id   TEXT,
			created_at        TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS participant (
			id             TEXT PRIMARY KEY,
			room_id        TEXT NOT NULL REFERENCES room(id) ON DELETE CASCADE,
			display_name   TEXT NOT NULL,
			session_token  TEXT NOT NULL UNIQUE,
			role           TEXT NOT NULL CHECK (role IN ('gm', 'player')),
			created_at     TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_participant_room ON participant(room_id);

		CREATE TABLE IF NOT EXISTS asset (
			id             TEXT PRIMARY KEY,
			content_hash   TEXT NOT NULL UNIQUE,
			filename       TEXT NOT NULL,
			mime_type      TEXT NOT NULL,
			byte_size      INTEGER NOT NULL,
			created_at     TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS scene (
			id             TEXT PRIMARY KEY,
			room_id        TEXT NOT NULL REFERENCES room(id) ON DELETE CASCADE,
			name           TEXT NOT NULL,
			map_asset_id   TEXT REFERENCES asset(id) ON DELETE SET NULL,
			grid_size      INTEGER NOT NULL DEFAULT 70,
			grid_offset_x  INTEGER NOT NULL DEFAULT 0,
			grid_offset_y  INTEGER NOT NULL DEFAULT 0,
			width          INTEGER NOT NULL DEFAULT 0,
			height         INTEGER NOT NULL DEFAULT 0,
			created_at     TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_scene_room ON scene(room_id);

		CREATE TABLE IF NOT EXISTS token (
			id                     TEXT PRIMARY KEY,
			scene_id               TEXT NOT NULL REFERENCES scene(id) ON DELETE CASCADE,
			name                   TEXT NOT NULL,
			image_asset_id         TEXT REFERENCES asset(id) ON DELETE SET NULL,
			x                      REAL NOT NULL DEFAULT 0,
			y                      REAL NOT NULL DEFAULT 0,
			width                  REAL NOT NULL DEFAULT 1,
			height                 REAL NOT NULL DEFAULT 1,
			owner_participant_id   TEXT REFERENCES participant(id) ON DELETE SET NULL,
			visibility             TEXT NOT NULL DEFAULT 'visible' CHECK (visibility IN ('visible', 'hidden'))
		);
		CREATE INDEX IF NOT EXISTS idx_token_scene ON token(scene_id);

		CREATE TABLE IF NOT EXISTS fog_cell (
			scene_id  TEXT NOT NULL REFERENCES scene(id) ON DELETE CASCADE,
			cell_x    INTEGER NOT NULL,
			cell_y    INTEGER NOT NULL,
			PRIMARY KEY (scene_id, cell_x, cell_y)
		);

		CREATE TABLE IF NOT EXISTS drawing (
			id                        TEXT PRIMARY KEY,
			scene_id                  TEXT NOT NULL REFERENCES scene(id) ON DELETE CASCADE,
			kind                      TEXT NOT NULL CHECK (kind IN ('freehand', 'line', 'rect', 'circle')),
			points                    TEXT NOT NULL,
			color                     TEXT NOT NULL,
			created_by_participant_id TEXT REFERENCES participant(id) ON DELETE SET NULL,
			created_at                TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_drawing_scene ON drawing(scene_id);

		CREATE TABLE IF NOT EXISTS message (
			id                 TEXT PRIMARY KEY,
			room_id            TEXT NOT NULL REFERENCES room(id) ON DELETE CASCADE,
			participant_id     TEXT REFERENCES participant(id) ON DELETE SET NULL,
			participant_name   TEXT NOT NULL,
			kind               TEXT NOT NULL CHECK (kind IN ('text', 'roll')),
			body               TEXT NOT NULL,
			roll_expression    TEXT,
			roll_result        INTEGER,
			roll_breakdown     TEXT,
			created_at         TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_message_room ON message(room_id);
	`)
	return err
}

// addColumnIfMissing is SQLite's missing "ALTER TABLE ... ADD COLUMN IF
// NOT EXISTS": it checks the table's current columns first, so applying
// the schema to an already-migrated database is a no-op. definition is
// the column type and constraints — everything the ALTER TABLE
// statement needs after the column name.
func (s *Store) addColumnIfMissing(table, column, definition string) error {
	var exists int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("inspect %s columns: %w", table, err)
	}
	if exists > 0 {
		return nil
	}

	// Table and column names can't be bound as parameters, so they're
	// interpolated — every caller is a compile-time constant in this
	// file, never user input.
	if _, err := s.db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, definition)); err != nil {
		return fmt.Errorf("add %s.%s column: %w", table, column, err)
	}
	return nil
}
