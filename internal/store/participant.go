package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"longtable/internal/auth"
)

type Role string

const (
	RoleGM     Role = "gm"
	RolePlayer Role = "player"
)

type Participant struct {
	ID           string
	RoomID       string
	DisplayName  string
	SessionToken string
	Role         Role
	CreatedAt    string
}

// execer is satisfied by both *sql.DB and *sql.Tx, so participant
// creation can run standalone (player join) or as part of a larger
// transaction (room creation).
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func createParticipant(e execer, roomID, displayName string, role Role) (Participant, error) {
	token, err := auth.NewToken()
	if err != nil {
		return Participant{}, err
	}

	p := Participant{
		ID:           uuid.NewString(),
		RoomID:       roomID,
		DisplayName:  displayName,
		SessionToken: token,
		Role:         role,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}

	_, err = e.ExecContext(context.Background(),
		`INSERT INTO participant (id, room_id, display_name, session_token, role, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.RoomID, p.DisplayName, p.SessionToken, string(p.Role), p.CreatedAt,
	)
	if err != nil {
		return Participant{}, err
	}
	return p, nil
}

// JoinRoom creates a new player participant in roomID.
func (s *Store) JoinRoom(roomID, displayName string) (Participant, error) {
	return createParticipant(s.db, roomID, displayName, RolePlayer)
}

// GMLogin creates a new GM participant in roomID. Called after the
// caller has already verified the room password.
func (s *Store) GMLogin(roomID, displayName string) (Participant, error) {
	return createParticipant(s.db, roomID, displayName, RoleGM)
}

// GetParticipantByToken resolves a session token to a participant, and
// confirms it belongs to roomID (tokens aren't valid across rooms).
func (s *Store) GetParticipantByToken(roomID, token string) (Participant, error) {
	var p Participant
	var role string
	err := s.db.QueryRow(
		`SELECT id, room_id, display_name, session_token, role, created_at
		 FROM participant WHERE room_id = ? AND session_token = ?`,
		roomID, token,
	).Scan(&p.ID, &p.RoomID, &p.DisplayName, &p.SessionToken, &role, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Participant{}, ErrNotFound
		}
		return Participant{}, err
	}
	p.Role = Role(role)
	return p, nil
}
