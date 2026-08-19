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

// ListScenesForRoom returns every scene in a room, oldest first, which
// is the order a GM built them in and so the order they expect to pick
// from.
//
// rowid settles a tie on created_at, which a clock about a millisecond
// wide leaves plenty of — see ListRecentMessages. A Scenes dialog that
// reordered itself between openings is the symptom.
func (s *Store) ListScenesForRoom(roomID string) ([]Scene, error) {
	rows, err := s.db.Query(
		`SELECT id, room_id, name, map_asset_id, grid_size, grid_offset_x, grid_offset_y, width, height, created_at
		 FROM scene WHERE room_id = ? ORDER BY created_at, rowid`, roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scenes []Scene
	for rows.Next() {
		var sc Scene
		if err := rows.Scan(&sc.ID, &sc.RoomID, &sc.Name, &sc.MapAssetID, &sc.GridSize,
			&sc.GridOffsetX, &sc.GridOffsetY, &sc.Width, &sc.Height, &sc.CreatedAt); err != nil {
			return nil, err
		}
		scenes = append(scenes, sc)
	}
	return scenes, rows.Err()
}

// DeleteScene removes a scene and everything drawn on it. The tokens,
// fog cells and drawings go with it through their `ON DELETE CASCADE`
// foreign keys rather than three deletes here — the connection sets
// `foreign_keys(ON)` (see internal/db), without which they would all
// silently survive as orphans.
//
// `room.active_scene_id` is deliberately *not* one of those foreign
// keys, so deleting the room's active scene leaves it pointing at
// nothing. The hub refuses that case rather than this repairing it: a
// room with no scene on screen and no way to say which should be is a
// worse state than a refusal.
func (s *Store) DeleteScene(sceneID string) error {
	_, err := s.db.Exec(`DELETE FROM scene WHERE id = ?`, sceneID)
	return err
}

// SetSceneMap swaps a scene's map image and the bounds that go with it,
// leaving everything placed on the scene alone. Dimensions travel with
// the image because they describe it — a new map at the old map's size
// would stretch, and fog cells are grid coordinates that stay valid
// either way.
func (s *Store) SetSceneMap(sceneID string, mapAssetID *string, width, height int) error {
	_, err := s.db.Exec(
		`UPDATE scene SET map_asset_id = ?, width = ?, height = ? WHERE id = ?`,
		mapAssetID, width, height, sceneID,
	)
	return err
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
