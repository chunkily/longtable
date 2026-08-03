// Package store owns Longtable's SQLite schema and the typed queries
// used by the REST API, the WebSocket hub, and the CLI admin commands.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
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
	if _, err := s.addColumnIfMissing("drawing", "created_by_participant_id",
		`TEXT REFERENCES participant(id) ON DELETE SET NULL`); err != nil {
		return err
	}
	if _, err := s.addColumnIfMissing("room_asset", "name", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if _, err := s.addColumnIfMissing("room_asset", "grid_size", `INTEGER`); err != nil {
		return err
	}
	if err := s.addAssetKindColumn(); err != nil {
		return err
	}
	return s.migrateCircleDrawingsToEllipse()
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
			kind                      TEXT NOT NULL CHECK (kind IN ('freehand', 'line', 'rect', 'ellipse')),
			points                    TEXT NOT NULL,
			color                     TEXT NOT NULL,
			created_by_participant_id TEXT REFERENCES participant(id) ON DELETE SET NULL,
			created_at                TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_drawing_scene ON drawing(scene_id);

		-- Which assets a room's library holds. The asset row itself stays
		-- global and content-addressed, so identical uploads share one
		-- stored file no matter which room they arrived in; this table is
		-- what keeps a room from *seeing* art it never added. Two rooms
		-- uploading the same map get one blob, one asset row, and a
		-- library entry each, with their own attribution text.
		CREATE TABLE IF NOT EXISTS room_asset (
			room_id      TEXT NOT NULL REFERENCES room(id) ON DELETE CASCADE,
			asset_id     TEXT NOT NULL REFERENCES asset(id) ON DELETE CASCADE,
			name         TEXT NOT NULL DEFAULT '',
			attribution  TEXT NOT NULL DEFAULT '',
			-- Pixels per grid square, as measured when the map was aligned on
			-- the assets page. Null for art nobody aligned, which is every
			-- token and any map added before this existed. Per-room rather
			-- than on the (content-addressed, globally shared) asset row: a
			-- global column would have one room's upload writing a value
			-- another room reads, and every rule for resolving that conflict
			-- is worse than measuring it twice.
			grid_size    INTEGER,
			-- What the room keeps this picture for: art that goes on a token,
			-- or a map that goes under one. Per-room for the same reason the
			-- name is — one group's boss portrait is another group's battle
			-- map, and the shared asset row can't hold both answers.
			kind         TEXT NOT NULL DEFAULT 'token' CHECK (kind IN ('token', 'map')),
			added_at     TEXT NOT NULL,
			PRIMARY KEY (room_id, asset_id)
		);
		CREATE INDEX IF NOT EXISTS idx_room_asset_room ON room_asset(room_id);

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

// addAssetKindColumn adds room_asset.kind and sorts the rows that
// predate it into the two kinds.
//
// The backfill runs only on the boot that adds the column, which is why
// this can't just be an addColumnIfMissing call. Whether a map was
// aligned is the only signal an old row carries about what it is, and
// it's a decent one — the assets page only ever offered alignment for
// maps — but it's a guess, and someone who corrects it afterwards must
// not have that correction re-guessed away on the next restart.
func (s *Store) addAssetKindColumn() error {
	added, err := s.addColumnIfMissing("room_asset", "kind",
		`TEXT NOT NULL DEFAULT 'token' CHECK (kind IN ('token', 'map'))`)
	if err != nil {
		return err
	}
	if !added {
		return nil
	}
	if _, err := s.db.Exec(`UPDATE room_asset SET kind = 'map' WHERE grid_size IS NOT NULL`); err != nil {
		return fmt.Errorf("classify pre-existing library assets: %w", err)
	}
	return nil
}

// migrateCircleDrawingsToEllipse replaces the old 'circle' drawing kind
// with 'ellipse'.
//
// The two differ in more than name: a circle was stored as its centre
// plus a point on its edge, while an ellipse — like a rect — is stored
// as two opposite corners of the box it's drawn in. Existing rows are
// converted so an old circle comes back as an ellipse of the same size
// in the same place, rather than as a degenerate sliver.
//
// SQLite can't ALTER a CHECK constraint, so changing the set of allowed
// kinds means rebuilding the table. Whether that's already happened is
// read from the table's own definition, which keeps this a no-op on a
// database that has been through it (and on a fresh one, created with
// the new constraint already in place).
func (s *Store) migrateCircleDrawingsToEllipse() error {
	var ddl string
	if err := s.db.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'drawing'`,
	).Scan(&ddl); err != nil {
		return fmt.Errorf("read drawing table definition: %w", err)
	}
	if !strings.Contains(ddl, "'circle'") {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin circle-to-ellipse migration: %w", err)
	}
	defer tx.Rollback()

	if err := convertCirclePoints(tx); err != nil {
		return err
	}

	// Rebuilding the table is the only way to restate its CHECK: copy
	// into a table with the new constraint, then take over the name.
	if _, err := tx.Exec(`
		CREATE TABLE drawing_migrated (
			id                        TEXT PRIMARY KEY,
			scene_id                  TEXT NOT NULL REFERENCES scene(id) ON DELETE CASCADE,
			kind                      TEXT NOT NULL CHECK (kind IN ('freehand', 'line', 'rect', 'ellipse')),
			points                    TEXT NOT NULL,
			color                     TEXT NOT NULL,
			created_by_participant_id TEXT REFERENCES participant(id) ON DELETE SET NULL,
			created_at                TEXT NOT NULL
		);

		INSERT INTO drawing_migrated (id, scene_id, kind, points, color, created_by_participant_id, created_at)
			SELECT id, scene_id, CASE kind WHEN 'circle' THEN 'ellipse' ELSE kind END,
			       points, color, created_by_participant_id, created_at
			FROM drawing;

		DROP TABLE drawing;
		ALTER TABLE drawing_migrated RENAME TO drawing;
		CREATE INDEX IF NOT EXISTS idx_drawing_scene ON drawing(scene_id);
	`); err != nil {
		return fmt.Errorf("rebuild drawing table: %w", err)
	}

	return tx.Commit()
}

// convertCirclePoints rewrites every circle's [centre, edge] geometry as
// the [top-left, bottom-right] corner pair an ellipse is stored as.
func convertCirclePoints(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT id, points FROM drawing WHERE kind = 'circle'`)
	if err != nil {
		return fmt.Errorf("list circle drawings: %w", err)
	}

	converted := make(map[string]string)
	for rows.Next() {
		var id, pointsJSON string
		if err := rows.Scan(&id, &pointsJSON); err != nil {
			rows.Close()
			return err
		}

		var points []Point
		if err := json.Unmarshal([]byte(pointsJSON), &points); err != nil || len(points) != 2 {
			// Not geometry this migration knows how to reinterpret; the
			// renderer already has to tolerate a malformed shape, so leave
			// it exactly as it is rather than guessing.
			continue
		}

		centre, edge := points[0], points[1]
		radius := math.Hypot(edge.X-centre.X, edge.Y-centre.Y)
		corners, err := json.Marshal([]Point{
			{X: centre.X - radius, Y: centre.Y - radius},
			{X: centre.X + radius, Y: centre.Y + radius},
		})
		if err != nil {
			rows.Close()
			return err
		}
		converted[id] = string(corners)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for id, points := range converted {
		if _, err := tx.Exec(`UPDATE drawing SET points = ? WHERE id = ?`, points, id); err != nil {
			return fmt.Errorf("convert circle geometry: %w", err)
		}
	}
	return nil
}

// addColumnIfMissing is SQLite's missing "ALTER TABLE ... ADD COLUMN IF
// NOT EXISTS": it checks the table's current columns first, so applying
// the schema to an already-migrated database is a no-op. definition is
// the column type and constraints — everything the ALTER TABLE
// statement needs after the column name.
//
// Reports whether it actually added the column, which is what a caller
// with rows to backfill needs: "the column is there" and "the column is
// there because I just made it" are different facts, and only the second
// one licenses rewriting existing rows.
func (s *Store) addColumnIfMissing(table, column, definition string) (bool, error) {
	var exists int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("inspect %s columns: %w", table, err)
	}
	if exists > 0 {
		return false, nil
	}

	// Table and column names can't be bound as parameters, so they're
	// interpolated — every caller is a compile-time constant in this
	// file, never user input.
	if _, err := s.db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, definition)); err != nil {
		return false, fmt.Errorf("add %s.%s column: %w", table, column, err)
	}
	return true, nil
}
