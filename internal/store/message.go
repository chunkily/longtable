package store

import (
	"time"

	"github.com/google/uuid"
)

type MessageKind string

const (
	MessageKindText MessageKind = "text"
	MessageKindRoll MessageKind = "roll"
)

// Message is a single entry in a room's chat log. Plain chat (Kind ==
// text) only uses Body; a /roll command (Kind == roll) additionally
// carries the roll fields — Body still holds the raw text that was
// typed, so the log can show exactly what the player sent.
type Message struct {
	ID              string
	RoomID          string
	ParticipantID   *string
	ParticipantName string
	Kind            MessageKind
	Body            string
	RollExpression  *string
	RollResult      *int
	RollBreakdown   *string
	CreatedAt       string
}

func (s *Store) InsertMessage(m Message) (Message, error) {
	m.ID = uuid.NewString()
	m.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(
		`INSERT INTO message (id, room_id, participant_id, participant_name, kind, body, roll_expression, roll_result, roll_breakdown, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.RoomID, m.ParticipantID, m.ParticipantName, string(m.Kind), m.Body,
		m.RollExpression, m.RollResult, m.RollBreakdown, m.CreatedAt,
	)
	if err != nil {
		return Message{}, err
	}
	return m, nil
}

// ListRecentMessages returns the most recent messages for a room,
// newest first, so a client can hydrate a bit of chat-log history on
// connect.
func (s *Store) ListRecentMessages(roomID string, limit int) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT id, room_id, participant_id, participant_name, kind, body, roll_expression, roll_result, roll_breakdown, created_at
		 FROM message WHERE room_id = ? ORDER BY created_at DESC LIMIT ?`,
		roomID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		var kind string
		if err := rows.Scan(&m.ID, &m.RoomID, &m.ParticipantID, &m.ParticipantName, &kind, &m.Body,
			&m.RollExpression, &m.RollResult, &m.RollBreakdown, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Kind = MessageKind(kind)
		messages = append(messages, m)
	}
	return messages, rows.Err()
}
