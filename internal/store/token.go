package store

import (
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

type Visibility string

const (
	VisibilityVisible Visibility = "visible"
	VisibilityHidden  Visibility = "hidden"
)

type Token struct {
	ID                 string
	SceneID            string
	Name               string
	ImageAssetID       *string
	X, Y               float64
	Width, Height      float64
	OwnerParticipantID *string
	Visibility         Visibility
}

func (s *Store) CreateToken(t Token) (Token, error) {
	t.ID = uuid.NewString()
	if t.Visibility == "" {
		t.Visibility = VisibilityVisible
	}
	_, err := s.db.Exec(
		`INSERT INTO token (id, scene_id, name, image_asset_id, x, y, width, height, owner_participant_id, visibility)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.SceneID, t.Name, t.ImageAssetID, t.X, t.Y, t.Width, t.Height, t.OwnerParticipantID, string(t.Visibility),
	)
	if err != nil {
		return Token{}, err
	}
	return t, nil
}

func (s *Store) MoveToken(id string, x, y float64) error {
	_, err := s.db.Exec(`UPDATE token SET x = ?, y = ? WHERE id = ?`, x, y, id)
	return err
}

// TokenRoomID returns the room a token belongs to (via its scene), so
// callers (the WS hub) can confirm a token referenced by ID actually
// belongs to the connection's room before acting on it.
func (s *Store) TokenRoomID(tokenID string) (string, error) {
	var roomID string
	err := s.db.QueryRow(
		`SELECT scene.room_id FROM token JOIN scene ON scene.id = token.scene_id WHERE token.id = ?`, tokenID,
	).Scan(&roomID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return roomID, nil
}

func (s *Store) ListTokensForScene(sceneID string) ([]Token, error) {
	rows, err := s.db.Query(
		`SELECT id, scene_id, name, image_asset_id, x, y, width, height, owner_participant_id, visibility
		 FROM token WHERE scene_id = ?`, sceneID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []Token
	for rows.Next() {
		var t Token
		var visibility string
		if err := rows.Scan(&t.ID, &t.SceneID, &t.Name, &t.ImageAssetID, &t.X, &t.Y, &t.Width, &t.Height, &t.OwnerParticipantID, &visibility); err != nil {
			return nil, err
		}
		t.Visibility = Visibility(visibility)
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}
