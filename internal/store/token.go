package store

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

type Visibility string

const (
	VisibilityVisible Visibility = "visible"
	VisibilityHidden  Visibility = "hidden"
)

// TrackerSlots is how many numeric trackers a token carries. Fixed
// rather than a growable list, following Roll20's token bars: three is
// what a character sheet's worth of at-a-glance numbers turns out to be
// (hit points, armour class, one resource), and a fixed count is what
// lets the map draw them in the same place on every token.
const TrackerSlots = 3

// Tracker is one of a token's numbered slots. Label is per token rather
// than per room — a monster's third slot is legendary resistances and a
// wizard's is spell slots, and there is no room-wide settings surface to
// hang a shared label off anyway.
//
// Value is a pointer because an empty slot and a slot reading zero are
// different things a GM will care about: a creature on 0 hit points is
// the whole point of tracking them. Integers rather than floats because
// every number on a D&D sheet is one, and a float would render as "12.5
// HP" the first time arithmetic went slightly wrong.
type Tracker struct {
	Label string `json:"label"`
	Value *int   `json:"value"`
}

type Token struct {
	ID                 string
	SceneID            string
	Name               string
	ImageAssetID       *string
	X, Y               float64
	Width, Height      float64
	OwnerParticipantID *string
	Visibility         Visibility
	// Always exactly TrackerSlots long once it has been through the store
	// — see normalizeTrackers. Conditions is free-form status text
	// ("Prone", "Concentrating"), in the order it was added.
	Trackers   []Tracker
	Conditions []string
}

// normalizeTrackers pads or truncates to exactly TrackerSlots, so a row
// written before trackers existed (an empty JSON array) and one written
// by a client sending too few both come back the same shape. Every
// caller can then index slots 0..2 without checking the length.
func normalizeTrackers(trackers []Tracker) []Tracker {
	out := make([]Tracker, TrackerSlots)
	copy(out, trackers)
	return out
}

// encodeTokenExtras marshals the two JSON-backed columns. Conditions is
// forced to an empty slice first: a nil one marshals to `null`, which
// the NOT NULL column would take and every reader would then have to
// treat as a third case alongside "[]" and a real list.
func encodeTokenExtras(t Token) (string, string, error) {
	trackers, err := json.Marshal(normalizeTrackers(t.Trackers))
	if err != nil {
		return "", "", err
	}
	conditions := t.Conditions
	if conditions == nil {
		conditions = []string{}
	}
	conditionsJSON, err := json.Marshal(conditions)
	if err != nil {
		return "", "", err
	}
	return string(trackers), string(conditionsJSON), nil
}

func decodeTokenExtras(t *Token, trackersJSON, conditionsJSON string) error {
	var trackers []Tracker
	if err := json.Unmarshal([]byte(trackersJSON), &trackers); err != nil {
		return err
	}
	t.Trackers = normalizeTrackers(trackers)
	t.Conditions = []string{}
	return json.Unmarshal([]byte(conditionsJSON), &t.Conditions)
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
	t.Trackers = normalizeTrackers(t.Trackers)
	if t.Conditions == nil {
		t.Conditions = []string{}
	}
	trackersJSON, conditionsJSON, err := encodeTokenExtras(t)
	if err != nil {
		return Token{}, err
	}
	_, err = s.db.Exec(
		`INSERT INTO token (id, scene_id, name, image_asset_id, x, y, width, height, owner_participant_id, visibility, trackers, conditions)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.SceneID, t.Name, t.ImageAssetID, t.X, t.Y, t.Width, t.Height, t.OwnerParticipantID, string(t.Visibility),
		trackersJSON, conditionsJSON,
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
	trackersJSON, conditionsJSON, err := encodeTokenExtras(t)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`UPDATE token SET name = ?, image_asset_id = ?, width = ?, height = ?, owner_participant_id = ?, visibility = ?,
		        trackers = ?, conditions = ?
		 WHERE id = ?`,
		t.Name, t.ImageAssetID, t.Width, t.Height, t.OwnerParticipantID, string(t.Visibility),
		trackersJSON, conditionsJSON, t.ID,
	)
	return err
}

// GetToken loads a single token by ID. The WS hub needs its scene before
// it can decide whether the caller is allowed to touch it, and its
// visibility before it can decide who is even told it's gone.
func (s *Store) GetToken(id string) (Token, error) {
	var t Token
	var visibility, trackersJSON, conditionsJSON string
	err := s.db.QueryRow(
		`SELECT id, scene_id, name, image_asset_id, x, y, width, height, owner_participant_id, visibility, trackers, conditions
		 FROM token WHERE id = ?`, id,
	).Scan(&t.ID, &t.SceneID, &t.Name, &t.ImageAssetID, &t.X, &t.Y, &t.Width, &t.Height, &t.OwnerParticipantID, &visibility,
		&trackersJSON, &conditionsJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Token{}, ErrNotFound
		}
		return Token{}, err
	}
	t.Visibility = Visibility(visibility)
	if err := decodeTokenExtras(&t, trackersJSON, conditionsJSON); err != nil {
		return Token{}, err
	}
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
		`SELECT id, scene_id, name, image_asset_id, x, y, width, height, owner_participant_id, visibility, trackers, conditions
		 FROM token WHERE scene_id = ?`, sceneID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []Token
	for rows.Next() {
		var t Token
		var visibility, trackersJSON, conditionsJSON string
		if err := rows.Scan(&t.ID, &t.SceneID, &t.Name, &t.ImageAssetID, &t.X, &t.Y, &t.Width, &t.Height, &t.OwnerParticipantID, &visibility,
			&trackersJSON, &conditionsJSON); err != nil {
			return nil, err
		}
		t.Visibility = Visibility(visibility)
		if err := decodeTokenExtras(&t, trackersJSON, conditionsJSON); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}
