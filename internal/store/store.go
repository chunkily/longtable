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
	if err := s.createTables(); err != nil {
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return s, nil
}

func (s *Store) createTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS room (
			id                   TEXT PRIMARY KEY,
			slug                 TEXT NOT NULL UNIQUE,
			name                 TEXT NOT NULL,
			gm_password_hash     TEXT NOT NULL,
			active_scene_id      TEXT,
			-- The room's first real setting. 0 is the open table Longtable
			-- has always been: anyone may drag anyone's token. Defaulted in
			-- the column rather than in Go so a row written by any path —
			-- including a future one — is open rather than accidentally
			-- locked, which is the direction that fails safely.
			owner_only_movement  INTEGER NOT NULL DEFAULT 0,
			-- The initiative tracker's turn and round. Two scalars rather
			-- than a table of their own: a room runs one encounter at a
			-- time, so this is one row's worth of state and a second table
			-- would be a join for two numbers. No foreign key on the entry
			-- id on purpose — an entry can vanish with its token, and
			-- GetInitiativeState reads a dangling pointer as "nobody's
			-- turn" rather than failing.
			initiative_round     INTEGER NOT NULL DEFAULT 1,
			initiative_entry_id  TEXT,
			created_at           TEXT NOT NULL
		);

		-- A participant is a *seat*: the durable half of an identity, and
		-- what every token's owner_participant_id points at. It carries no
		-- credential — see the session table below, and ADR-0008.
		CREATE TABLE IF NOT EXISTS participant (
			id             TEXT PRIMARY KEY,
			room_id        TEXT NOT NULL REFERENCES room(id) ON DELETE CASCADE,
			display_name   TEXT NOT NULL,
			role           TEXT NOT NULL CHECK (role IN ('gm', 'player')),
			created_at     TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_participant_room ON participant(room_id);

		-- One row per device that has taken a seat. Many-to-one over time:
		-- a phone and a laptop on the same seat are two sessions and one
		-- person, and clearing browser data costs a session rather than an
		-- identity. Deleting a seat takes its sessions with it.
		CREATE TABLE IF NOT EXISTS session (
			token           TEXT PRIMARY KEY,
			participant_id  TEXT NOT NULL REFERENCES participant(id) ON DELETE CASCADE,
			created_at      TEXT NOT NULL,
			last_seen_at    TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_session_participant ON session(participant_id);

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
			visibility             TEXT NOT NULL DEFAULT 'visible' CHECK (visibility IN ('visible', 'hidden')),
			trackers               TEXT NOT NULL DEFAULT '[]',
			conditions             TEXT NOT NULL DEFAULT '[]'
		);
		CREATE INDEX IF NOT EXISTS idx_token_scene ON token(scene_id);

		-- Fog is the set of *hidden* cells, 32 of them packed into each
		-- row's mask: bit n is the cell at x = chunk_x*32 + n. An absent
		-- row means those 32 cells are revealed, which is what makes a
		-- fresh scene — revealed everywhere — cost no rows at all. See
		-- internal/store/fog.go for why it's stored this way round.
		CREATE TABLE IF NOT EXISTS fog_mask (
			scene_id  TEXT NOT NULL REFERENCES scene(id) ON DELETE CASCADE,
			cell_y    INTEGER NOT NULL,
			chunk_x   INTEGER NOT NULL,
			mask      INTEGER NOT NULL,
			PRIMARY KEY (scene_id, cell_y, chunk_x)
		);
		-- fog_cell held one row per *revealed* cell, the inverse of the
		-- above and 32x the rows. Dropped rather than migrated: it stored
		-- the complement of what fog_mask stores, so every row would have
		-- to be inverted against scene bounds that unbounded scenes don't
		-- have. Painted fog in an existing database is lost on upgrade.
		DROP TABLE IF EXISTS fog_cell;

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

		-- The turn order. Hung off the *room* rather than a scene, so a GM
		-- flipping to the battle map mid-fight doesn't lose the encounter.
		--
		-- token_id is nullable: an entry can stand alone (a lair action, a
		-- hazard, a creature nobody has drawn). When it does point at a
		-- token, ON DELETE CASCADE takes the entry with it — a combatant
		-- taken off the map has left the fight, and an entry pointing at
		-- nothing would have no name to show.
		CREATE TABLE IF NOT EXISTS initiative_entry (
			id          TEXT PRIMARY KEY,
			room_id     TEXT NOT NULL REFERENCES room(id) ON DELETE CASCADE,
			token_id    TEXT REFERENCES token(id) ON DELETE CASCADE,
			name        TEXT NOT NULL,
			initiative  REAL NOT NULL,
			hidden      INTEGER NOT NULL DEFAULT 0,
			sort_order  INTEGER NOT NULL DEFAULT 0,
			created_at  TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_initiative_room ON initiative_entry(room_id);

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
			created_at         TEXT NOT NULL,
			deleted_at         TEXT,
			deleted_by_participant_id TEXT REFERENCES participant(id) ON DELETE SET NULL
		);
		CREATE INDEX IF NOT EXISTS idx_message_room ON message(room_id);
	`)
	return err
}
