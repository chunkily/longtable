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
			id         TEXT PRIMARY KEY,
			scene_id   TEXT NOT NULL REFERENCES scene(id) ON DELETE CASCADE,
			kind       TEXT NOT NULL CHECK (kind IN ('freehand', 'line', 'rect', 'circle')),
			points     TEXT NOT NULL,
			color      TEXT NOT NULL,
			created_at TEXT NOT NULL
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
