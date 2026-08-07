package store

import (
	"database/sql"
	"errors"
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
//
// DeletedAt set means the first of the two delete stages has happened.
// SoftDeleteMessage deliberately leaves Body and the roll fields alone —
// the WS hub redacts them per viewer instead (see messagePayload in
// internal/ws/state.go), the same way a hidden token is withheld from
// Players rather than scrubbed from the row. DeletedByParticipantID
// tracks who did the deleting, which is not always ParticipantID: a GM
// moderating someone else's message is a second person who should still
// see what they removed. Both the author and the deleter keep seeing
// the original content (struck through, client-side); everyone else
// gets the generic placeholder. A second delete purges the row for
// real, at which point none of this matters any more.
type Message struct {
	ID                     string
	RoomID                 string
	ParticipantID          *string
	ParticipantName        string
	Kind                   MessageKind
	Body                   string
	RollExpression         *string
	RollResult             *int
	RollBreakdown          *string
	CreatedAt              string
	DeletedAt              *string
	DeletedByParticipantID *string
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
// connect. A soft-deleted message is included with its original content
// intact — redacting it per viewer is the WS layer's job, since this
// query has no notion of who's asking — and a purged one is gone from
// the table entirely and simply isn't here.
func (s *Store) ListRecentMessages(roomID string, limit int) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT id, room_id, participant_id, participant_name, kind, body, roll_expression, roll_result, roll_breakdown, created_at, deleted_at, deleted_by_participant_id
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
			&m.RollExpression, &m.RollResult, &m.RollBreakdown, &m.CreatedAt, &m.DeletedAt, &m.DeletedByParticipantID); err != nil {
			return nil, err
		}
		m.Kind = MessageKind(kind)
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// GetMessage loads a single message by ID — the WS hub needs its room
// and author before it can decide whether the caller may delete or
// purge it.
func (s *Store) GetMessage(id string) (Message, error) {
	var m Message
	var kind string
	err := s.db.QueryRow(
		`SELECT id, room_id, participant_id, participant_name, kind, body, roll_expression, roll_result, roll_breakdown, created_at, deleted_at, deleted_by_participant_id
		 FROM message WHERE id = ?`, id,
	).Scan(&m.ID, &m.RoomID, &m.ParticipantID, &m.ParticipantName, &kind, &m.Body,
		&m.RollExpression, &m.RollResult, &m.RollBreakdown, &m.CreatedAt, &m.DeletedAt, &m.DeletedByParticipantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Message{}, ErrNotFound
		}
		return Message{}, err
	}
	m.Kind = MessageKind(kind)
	return m, nil
}

// SoftDeleteMessage is the first delete stage: it stamps DeletedAt and
// DeletedByParticipantID but leaves the content columns untouched — the
// row still holds the real message, because the author and whoever just
// deleted it are still allowed to see it (struck through) after this.
// The row survives so a second delete still has ParticipantID to check
// authorship against, same as before; DeletedByParticipantID additionally
// needs to survive because the deleter isn't always the author.
func (s *Store) SoftDeleteMessage(id, deletedByParticipantID string) error {
	_, err := s.db.Exec(
		`UPDATE message SET deleted_at = ?, deleted_by_participant_id = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), deletedByParticipantID, id,
	)
	return err
}

// DeleteMessage is the second delete stage: it removes the row
// outright. Deleting one that's already gone is not an error, matching
// DeleteDrawing — two people purging the same message at once
// shouldn't have one of them fail.
func (s *Store) DeleteMessage(id string) error {
	_, err := s.db.Exec(`DELETE FROM message WHERE id = ?`, id)
	return err
}
