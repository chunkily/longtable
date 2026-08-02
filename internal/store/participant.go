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

// ListParticipantsForRoom returns everyone who has ever joined the room,
// oldest first. That is the roster, and it is a different question from
// who is connected right now — connectivity is only ever in the hub's
// memory and is never written down. A Player who joined last week and is
// offline today is still someone a GM can hand a token to.
//
// Deliberately does not select session_token. It is a credential, this
// list is the basis of something broadcast to the whole room, and the
// surest way for a payload builder to never leak it is for it never to
// be loaded in the first place. The field comes back zero.
func (s *Store) ListParticipantsForRoom(roomID string) ([]Participant, error) {
	rows, err := s.db.Query(
		`SELECT id, room_id, display_name, role, created_at
		 FROM participant WHERE room_id = ? ORDER BY created_at ASC`, roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []Participant
	for rows.Next() {
		var p Participant
		var role string
		if err := rows.Scan(&p.ID, &p.RoomID, &p.DisplayName, &role, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.Role = Role(role)
		participants = append(participants, p)
	}
	return participants, rows.Err()
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
