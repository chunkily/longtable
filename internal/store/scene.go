package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Scene struct {
	ID          string
	RoomID      string
	Name        string
	MapAssetID  *string
	GridSize    int
	GridOffsetX int
	GridOffsetY int
	Width       int
	Height      int
	CreatedAt   string
}

func (s *Store) CreateScene(roomID, name string, mapAssetID *string, gridSize, width, height int) (Scene, error) {
	sc := Scene{
		ID:         uuid.NewString(),
		RoomID:     roomID,
		Name:       name,
		MapAssetID: mapAssetID,
		GridSize:   gridSize,
		Width:      width,
		Height:     height,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}
	_, err := s.db.Exec(
		`INSERT INTO scene (id, room_id, name, map_asset_id, grid_size, width, height, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sc.ID, sc.RoomID, sc.Name, sc.MapAssetID, sc.GridSize, sc.Width, sc.Height, sc.CreatedAt,
	)
	if err != nil {
		return Scene{}, err
	}
	return sc, nil
}

// SceneRoomID returns the room a scene belongs to, so callers (the WS
// hub) can confirm a scene referenced by ID actually belongs to the
// connection's room before acting on it.
func (s *Store) SceneRoomID(sceneID string) (string, error) {
	var roomID string
	err := s.db.QueryRow(`SELECT room_id FROM scene WHERE id = ?`, sceneID).Scan(&roomID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return roomID, nil
}

func (s *Store) GetScene(id string) (Scene, error) {
	var sc Scene
	err := s.db.QueryRow(
		`SELECT id, room_id, name, map_asset_id, grid_size, grid_offset_x, grid_offset_y, width, height, created_at
		 FROM scene WHERE id = ?`, id,
	).Scan(&sc.ID, &sc.RoomID, &sc.Name, &sc.MapAssetID, &sc.GridSize, &sc.GridOffsetX, &sc.GridOffsetY, &sc.Width, &sc.Height, &sc.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Scene{}, ErrNotFound
		}
		return Scene{}, err
	}
	return sc, nil
}
