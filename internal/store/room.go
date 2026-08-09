package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"longtable/internal/auth"
)

var ErrNotFound = errors.New("not found")

// Seats are open-claim, but the GM's is a role boundary rather than an
// identity one: it goes through the room password instead. See ADR-0008.
var ErrGMSeatNeedsPassword = errors.New("the GM seat needs the room password")

// The room password signs you into the GM seat, so removing it would
// strand the only role that could undo the damage.
var ErrCannotDeleteGMSeat = errors.New("the GM seat cannot be removed")

type Room struct {
	ID             string
	Slug           string
	Name           string
	GMPasswordHash string
	ActiveSceneID  *string
	// When set, only a token's owner (and the GM) may move it. Off by
	// default: an open table is what Longtable has always been, and
	// ADR-0007 is the reason it stays the default rather than becoming
	// one.
	OwnerOnlyMovement bool
	CreatedAt         string
}

// roomColumns is the select list every room read shares, so a new column
// can't be added to one query and forgotten in another — which is
// exactly how a setting ends up reading as its zero value on one path.
const roomColumns = `id, slug, name, gm_password_hash, active_scene_id, owner_only_movement, created_at`

// CreateRoom creates a room and its founding GM participant in one
// transaction, retrying on the rare slug collision.
func (s *Store) CreateRoom(name, gmDisplayName, password string) (Room, Participant, error) {
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return Room{}, Participant{}, fmt.Errorf("hash password: %w", err)
	}

	const maxAttempts = 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		slug, err := newSlug()
		if err != nil {
			return Room{}, Participant{}, fmt.Errorf("generate slug: %w", err)
		}

		room, participant, err := s.tryCreateRoom(slug, name, passwordHash, gmDisplayName)
		if err == nil {
			return room, participant, nil
		}
		if !isUniqueConstraintErr(err) {
			return Room{}, Participant{}, err
		}
		// slug collision — try again with a fresh one
	}

	return Room{}, Participant{}, fmt.Errorf("could not generate a unique room slug after %d attempts", maxAttempts)
}

func (s *Store) tryCreateRoom(slug, name, passwordHash, gmDisplayName string) (Room, Participant, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Room{}, Participant{}, err
	}
	defer tx.Rollback()

	room := Room{
		ID:             uuid.NewString(),
		Slug:           slug,
		Name:           name,
		GMPasswordHash: passwordHash,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}
	if _, err := tx.Exec(
		`INSERT INTO room (id, slug, name, gm_password_hash, created_at) VALUES (?, ?, ?, ?, ?)`,
		room.ID, room.Slug, room.Name, room.GMPasswordHash, room.CreatedAt,
	); err != nil {
		return Room{}, Participant{}, err
	}

	participant, err := createParticipant(tx, room.ID, gmDisplayName, RoleGM)
	if err != nil {
		return Room{}, Participant{}, err
	}

	if err := tx.Commit(); err != nil {
		return Room{}, Participant{}, err
	}

	return room, participant, nil
}

func (s *Store) GetRoomBySlug(slug string) (Room, error) {
	return s.scanRoom(s.db.QueryRow(`SELECT `+roomColumns+` FROM room WHERE slug = ?`, slug))
}

func (s *Store) GetRoomByID(id string) (Room, error) {
	return s.scanRoom(s.db.QueryRow(`SELECT `+roomColumns+` FROM room WHERE id = ?`, id))
}

func (s *Store) scanRoom(row *sql.Row) (Room, error) {
	var r Room
	if err := row.Scan(
		&r.ID, &r.Slug, &r.Name, &r.GMPasswordHash, &r.ActiveSceneID, &r.OwnerOnlyMovement, &r.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Room{}, ErrNotFound
		}
		return Room{}, err
	}
	return r, nil
}

// ListRooms returns every room, most recently created first.
func (s *Store) ListRooms() ([]Room, error) {
	rows, err := s.db.Query(`SELECT ` + roomColumns + ` FROM room ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []Room
	for rows.Next() {
		var r Room
		if err := rows.Scan(
			&r.ID, &r.Slug, &r.Name, &r.GMPasswordHash, &r.ActiveSceneID, &r.OwnerOnlyMovement, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		rooms = append(rooms, r)
	}
	return rooms, rows.Err()
}

// SetOwnerOnlyMovement turns the room's movement lock on or off.
func (s *Store) SetOwnerOnlyMovement(roomID string, ownerOnly bool) error {
	_, err := s.db.Exec(`UPDATE room SET owner_only_movement = ? WHERE id = ?`, ownerOnly, roomID)
	return err
}

// SetGMPassword overwrites a room's GM password (used by both the
// gm-login recovery flow's admin path and the CLI reset-password
// command).
func (s *Store) SetGMPassword(roomID, newPassword string) error {
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, err = s.db.Exec(`UPDATE room SET gm_password_hash = ? WHERE id = ?`, hash, roomID)
	return err
}

// SetActiveScene marks sceneID as the room's live scene.
func (s *Store) SetActiveScene(roomID, sceneID string) error {
	_, err := s.db.Exec(`UPDATE room SET active_scene_id = ? WHERE id = ?`, sceneID, roomID)
	return err
}

func isUniqueConstraintErr(err error) bool {
	// modernc.org/sqlite wraps the sqlite3 result code in its error
	// message; there's no typed sentinel to compare against, so match
	// on the standard SQLite error text.
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
