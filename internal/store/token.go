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

// CreateToken persists t. An empty ID gets one generated; a caller that
// supplies one is restoring a token it just deleted, which has to come
// back under the id the rest of the room is still holding rather than as
// a new token that merely looks the same. Uniqueness is the primary
// key's job either way, so a duplicate is an error here rather than
// something to paper over — the same bargain CreateDrawing makes.
func (s *Store) CreateToken(t Token) (Token, error) {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
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

// UpdateToken rewrites a token's editable properties from t, keyed by
// t.ID. Position is deliberately not among them: moving is token.move's
// job, and folding it in here would let an edit dialog opened before a
// drag undo the drag when it was submitted after one.
//
// Every column it touches is written, so callers pass a whole token
// rather than a patch — load it, change what changed, hand it back. That
// is what keeps a field nobody's editing yet (an owner, one day an HP)
// from being quietly nulled by a form that didn't know about it.
func (s *Store) UpdateToken(t Token) error {
	_, err := s.db.Exec(
		`UPDATE token SET name = ?, image_asset_id = ?, width = ?, height = ?, owner_participant_id = ?, visibility = ?
		 WHERE id = ?`,
		t.Name, t.ImageAssetID, t.Width, t.Height, t.OwnerParticipantID, string(t.Visibility), t.ID,
	)
	return err
}

// GetToken loads a single token by ID. The WS hub needs its scene before
// it can decide whether the caller is allowed to touch it, and its
// visibility before it can decide who is even told it's gone.
func (s *Store) GetToken(id string) (Token, error) {
	var t Token
	var visibility string
	err := s.db.QueryRow(
		`SELECT id, scene_id, name, image_asset_id, x, y, width, height, owner_participant_id, visibility
		 FROM token WHERE id = ?`, id,
	).Scan(&t.ID, &t.SceneID, &t.Name, &t.ImageAssetID, &t.X, &t.Y, &t.Width, &t.Height, &t.OwnerParticipantID, &visibility)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Token{}, ErrNotFound
		}
		return Token{}, err
	}
	t.Visibility = Visibility(visibility)
	return t, nil
}

// DeleteToken removes a token permanently. Deleting one that's already
// gone is not an error, the same as DeleteDrawing: the caller has
// already read the row it means, and racing with another GM shouldn't
// fail the second one.
func (s *Store) DeleteToken(id string) error {
	_, err := s.db.Exec(`DELETE FROM token WHERE id = ?`, id)
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
