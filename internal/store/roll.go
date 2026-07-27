package store

import (
	"time"

	"github.com/google/uuid"
)

type Roll struct {
	ID              string
	RoomID          string
	SceneID         *string
	ParticipantID   *string
	ParticipantName string
	Expression      string
	Result          int
	Breakdown       string
	CreatedAt       string
}

func (s *Store) InsertRoll(r Roll) (Roll, error) {
	r.ID = uuid.NewString()
	r.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(
		`INSERT INTO roll (id, room_id, scene_id, participant_id, participant_name, expression, result, breakdown, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.RoomID, r.SceneID, r.ParticipantID, r.ParticipantName, r.Expression, r.Result, r.Breakdown, r.CreatedAt,
	)
	if err != nil {
		return Roll{}, err
	}
	return r, nil
}

// ListRecentRolls returns the most recent rolls for a room, newest
// first, so a client can hydrate a bit of chat-log history on connect.
func (s *Store) ListRecentRolls(roomID string, limit int) ([]Roll, error) {
	rows, err := s.db.Query(
		`SELECT id, room_id, scene_id, participant_id, participant_name, expression, result, breakdown, created_at
		 FROM roll WHERE room_id = ? ORDER BY created_at DESC LIMIT ?`,
		roomID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rolls []Roll
	for rows.Next() {
		var r Roll
		if err := rows.Scan(&r.ID, &r.RoomID, &r.SceneID, &r.ParticipantID, &r.ParticipantName, &r.Expression, &r.Result, &r.Breakdown, &r.CreatedAt); err != nil {
			return nil, err
		}
		rolls = append(rolls, r)
	}
	return rolls, rows.Err()
}
