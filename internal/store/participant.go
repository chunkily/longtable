package store

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"longtable/internal/auth"
)

type Role string

const (
	RoleGM     Role = "gm"
	RolePlayer Role = "player"
)

// Participant is a *seat*: the durable half of an identity in one room,
// and what every token's owner_participant_id points at. It carries no
// credential — a device proves it holds this seat with a session token,
// and many sessions can point at one seat over time. See ADR-0008.
//
// SessionToken is not a column. It is filled in only by the calls that
// mint a session (JoinRoom, GMLogin, ClaimSeat), as the token to hand
// back to that one caller; every other read leaves it empty.
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

// createSeat writes the durable half only. Callers that need a device
// signed in to it call issueSession afterwards; a GM adding an empty
// seat before anyone arrives deliberately doesn't.
func createSeat(e execer, roomID, displayName string, role Role) (Participant, error) {
	p := Participant{
		ID:          uuid.NewString(),
		RoomID:      roomID,
		DisplayName: displayName,
		Role:        role,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}

	_, err := e.ExecContext(context.Background(),
		`INSERT INTO participant (id, room_id, display_name, role, created_at) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.RoomID, p.DisplayName, string(p.Role), p.CreatedAt,
	)
	if err != nil {
		return Participant{}, err
	}
	return p, nil
}

// issueSession mints a token for a device taking participantID's seat.
func issueSession(e execer, participantID string) (string, error) {
	token, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = e.ExecContext(context.Background(),
		`INSERT INTO session (token, participant_id, created_at, last_seen_at) VALUES (?, ?, ?, ?)`,
		token, participantID, now, now,
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

func createParticipant(e execer, roomID, displayName string, role Role) (Participant, error) {
	p, err := createSeat(e, roomID, displayName, role)
	if err != nil {
		return Participant{}, err
	}
	token, err := issueSession(e, p.ID)
	if err != nil {
		return Participant{}, err
	}
	p.SessionToken = token
	return p, nil
}

// JoinRoom creates a new player seat in roomID and signs this device
// into it — the "I'm new here" path.
func (s *Store) JoinRoom(roomID, displayName string) (Participant, error) {
	return createParticipant(s.db, roomID, displayName, RolePlayer)
}

// GMLogin signs a device into the room's GM seat, creating one only if
// the room somehow has none. Reusing the seat is the point: before
// seats, every GM login minted a *new* participant, so a GM on a second
// device was a second person and the roster grew one entry per login.
// Called after the caller has already verified the room password —
// which is a role boundary, not an identity one (ADR-0007).
func (s *Store) GMLogin(roomID, displayName string) (Participant, error) {
	var p Participant
	var role string
	err := s.db.QueryRow(
		`SELECT id, room_id, display_name, role, created_at
		 FROM participant WHERE room_id = ? AND role = 'gm' ORDER BY created_at ASC LIMIT 1`,
		roomID,
	).Scan(&p.ID, &p.RoomID, &p.DisplayName, &role, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return createParticipant(s.db, roomID, displayName, RoleGM)
	}
	if err != nil {
		return Participant{}, err
	}
	p.Role = Role(role)

	token, err := issueSession(s.db, p.ID)
	if err != nil {
		return Participant{}, err
	}
	p.SessionToken = token
	return p, nil
}

// ClaimSeat signs a device into an existing seat. No secret and no
// approval: seats are open-claim, bounded by the fact that reaching one
// means holding the room's link (ADR-0007). The GM seat is the
// exception and goes through GMLogin's password instead.
func (s *Store) ClaimSeat(roomID, participantID string) (Participant, error) {
	var p Participant
	var role string
	err := s.db.QueryRow(
		`SELECT id, room_id, display_name, role, created_at
		 FROM participant WHERE room_id = ? AND id = ?`,
		roomID, participantID,
	).Scan(&p.ID, &p.RoomID, &p.DisplayName, &role, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Participant{}, ErrNotFound
		}
		return Participant{}, err
	}
	p.Role = Role(role)
	if p.Role == RoleGM {
		return Participant{}, ErrGMSeatNeedsPassword
	}

	token, err := issueSession(s.db, p.ID)
	if err != nil {
		return Participant{}, err
	}
	p.SessionToken = token
	return p, nil
}

// CreateSeat adds an empty seat, for a GM setting the table before
// anyone arrives. Nobody is signed into it until someone claims it.
func (s *Store) CreateSeat(roomID, displayName string) (Participant, error) {
	return createSeat(s.db, roomID, displayName, RolePlayer)
}

// DeleteSeat removes a seat and, by cascade, every session on it.
// Refuses the GM's own seat: the room password signs you into that one,
// so deleting it mid-session would strand the only role that can undo
// the damage.
func (s *Store) DeleteSeat(roomID, participantID string) error {
	var role string
	err := s.db.QueryRow(
		`SELECT role FROM participant WHERE room_id = ? AND id = ?`, roomID, participantID,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if Role(role) == RoleGM {
		return ErrCannotDeleteGMSeat
	}

	_, err = s.db.Exec(`DELETE FROM participant WHERE room_id = ? AND id = ?`, roomID, participantID)
	return err
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

// Seat is a room's seat as the *pre-join* screen sees it — someone who
// has the room's link but no session yet, and so has not proved anything
// about who they are.
//
// Deliberately narrow. It carries no session token, no created_at and
// nothing about the tokens the seat owns: the endpoint serving it is
// reachable by anyone holding the link, which is fine for "which chairs
// are at this table" and would not be for anything else. Sessions counts
// devices signed in over time, which is not the same question as whether
// anyone is at the table right now — that one is live presence and only
// the hub knows it.
type Seat struct {
	ID          string
	DisplayName string
	Role        Role
	Sessions    int
}

// ListSeatsForRoom returns the room's seats, oldest first — the order
// people sat down in, which is stabler than sorting by name.
func (s *Store) ListSeatsForRoom(roomID string) ([]Seat, error) {
	rows, err := s.db.Query(
		`SELECT p.id, p.display_name, p.role, COUNT(s.token)
		 FROM participant p LEFT JOIN session s ON s.participant_id = p.id
		 WHERE p.room_id = ?
		 GROUP BY p.id, p.display_name, p.role, p.created_at
		 ORDER BY p.created_at ASC`, roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seats []Seat
	for rows.Next() {
		var seat Seat
		var role string
		if err := rows.Scan(&seat.ID, &seat.DisplayName, &role, &seat.Sessions); err != nil {
			return nil, err
		}
		seat.Role = Role(role)
		seats = append(seats, seat)
	}
	return seats, rows.Err()
}

// ParticipantInRoom reports whether a participant belongs to a room.
//
// This is the check that stands between a token and someone in another
// room. Participant IDs are unguessable UUIDs, but so are asset IDs, and
// the same reasoning applies (see AssetInRoom): unguessable is not
// scoped, and one leaked ID shouldn't be assignable from anywhere.
func (s *Store) ParticipantInRoom(roomID, participantID string) (bool, error) {
	var one int
	err := s.db.QueryRow(
		`SELECT 1 FROM participant WHERE room_id = ? AND id = ?`, roomID, participantID,
	).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetParticipantByToken resolves a session token to the seat holding it,
// and confirms that seat belongs to roomID (tokens aren't valid across
// rooms). This is the one place a credential turns into an identity —
// join, reconnect, the asset endpoints and the WebSocket handshake all
// come through here.
//
// The returned SessionToken is left empty: the caller supplied the token
// and doesn't need it echoed, and a struct that stops carrying
// credentials around is one fewer thing a payload builder can leak.
func (s *Store) GetParticipantByToken(roomID, token string) (Participant, error) {
	var p Participant
	var role string
	err := s.db.QueryRow(
		`SELECT p.id, p.room_id, p.display_name, p.role, p.created_at
		 FROM participant p JOIN session s ON s.participant_id = p.id
		 WHERE p.room_id = ? AND s.token = ?`,
		roomID, token,
	).Scan(&p.ID, &p.RoomID, &p.DisplayName, &role, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Participant{}, ErrNotFound
		}
		return Participant{}, err
	}
	p.Role = Role(role)
	return p, nil
}

// TouchSession records that a token was used just now. Best-effort: the
// timestamp is for a Host looking at the database, and nothing in the
// app reads it, so a failure here must never fail the request it
// happened during.
func (s *Store) TouchSession(token string) {
	if _, err := s.db.Exec(
		`UPDATE session SET last_seen_at = ? WHERE token = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), token,
	); err != nil {
		slog.Warn("store: touch session failed", "error", err)
	}
}

// DeleteSession signs one device out, leaving the seat and every other
// device on it alone. This is what "leave room" spends, which is why
// leaving is cheap: it costs a session, never an identity.
func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM session WHERE token = ?`, token)
	return err
}
