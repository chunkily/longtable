package store

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type DrawingKind string

const (
	DrawingKindFreehand DrawingKind = "freehand"
	DrawingKindLine     DrawingKind = "line"
	DrawingKindRect     DrawingKind = "rect"
	DrawingKindCircle   DrawingKind = "circle"
)

// Point is a single (x, y) in a scene's world pixel space — unlike
// token/fog coordinates, drawings aren't grid-snapped, since freehand
// strokes and rubber-banded shapes are drawn freely.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Drawing is a persistent map annotation. Points holds however many
// coordinates the Kind needs: many for freehand strokes, exactly two
// (start/end, or two opposite corners, or center+edge) for the other
// kinds — interpreting that shape is up to the renderer.
type Drawing struct {
	ID        string
	SceneID   string
	Kind      DrawingKind
	Points    []Point
	Color     string
	CreatedAt string
}

func (s *Store) CreateDrawing(sceneID string, kind DrawingKind, points []Point, color string) (Drawing, error) {
	pointsJSON, err := json.Marshal(points)
	if err != nil {
		return Drawing{}, err
	}

	d := Drawing{
		ID:        uuid.NewString(),
		SceneID:   sceneID,
		Kind:      kind,
		Points:    points,
		Color:     color,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	_, err = s.db.Exec(
		`INSERT INTO drawing (id, scene_id, kind, points, color, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		d.ID, d.SceneID, string(d.Kind), string(pointsJSON), d.Color, d.CreatedAt,
	)
	if err != nil {
		return Drawing{}, err
	}
	return d, nil
}

// ListDrawingsForScene returns a scene's drawings in creation order, so
// later strokes render on top of earlier ones.
func (s *Store) ListDrawingsForScene(sceneID string) ([]Drawing, error) {
	rows, err := s.db.Query(
		`SELECT id, scene_id, kind, points, color, created_at FROM drawing WHERE scene_id = ? ORDER BY created_at ASC`,
		sceneID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var drawings []Drawing
	for rows.Next() {
		var d Drawing
		var kind, pointsJSON string
		if err := rows.Scan(&d.ID, &d.SceneID, &kind, &pointsJSON, &d.Color, &d.CreatedAt); err != nil {
			return nil, err
		}
		d.Kind = DrawingKind(kind)
		if err := json.Unmarshal([]byte(pointsJSON), &d.Points); err != nil {
			return nil, err
		}
		drawings = append(drawings, d)
	}
	return drawings, rows.Err()
}
