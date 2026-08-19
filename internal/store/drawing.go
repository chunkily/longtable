package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type DrawingKind string

const (
	DrawingKindFreehand DrawingKind = "freehand"
	DrawingKindLine     DrawingKind = "line"
	DrawingKindRect     DrawingKind = "rect"
	DrawingKindEllipse  DrawingKind = "ellipse"
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
// for the other kinds — a line's start and end, or two opposite corners
// of the box a rect or ellipse is drawn in. Interpreting that shape is
// up to the renderer.
//
// CreatedByParticipantID records who drew it, which is what lets an
// eraser distinguish "my own drawing" from someone else's. It's a
// pointer because it's nil for drawings made before authorship was
// tracked, and for those whose author has since been removed from the
// room (the column is ON DELETE SET NULL) — treat nil as "author
// unknown", not as "everyone's".
// Filled shades a rect's or an ellipse's interior as well as outlining
// it. Only those two kinds enclose an area, and the hub forces it false
// for the other two rather than storing a flag nothing can render.
//
// StrokeWidth is in world pixels, the same units Points are in, so a
// stroke keeps its thickness relative to the map at every zoom level.
type Drawing struct {
	ID                     string
	SceneID                string
	Kind                   DrawingKind
	Points                 []Point
	Color                  string
	Filled                 bool
	StrokeWidth            float64
	CreatedByParticipantID *string
	CreatedAt              string
}

// CreateDrawing persists d, filling in its CreatedAt. An empty ID gets
// one generated; a caller that supplies one is the client drawing
// optimistically, having already rendered the stroke under that id and
// needing to recognise it when the broadcast comes back. Uniqueness is
// the primary key's job either way, so a duplicate is an error here
// rather than something to paper over.
func (s *Store) CreateDrawing(d Drawing) (Drawing, error) {
	pointsJSON, err := json.Marshal(d.Points)
	if err != nil {
		return Drawing{}, err
	}

	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	d.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)

	_, err = s.db.Exec(
		`INSERT INTO drawing (id, scene_id, kind, points, color, filled, stroke_width, created_by_participant_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.SceneID, string(d.Kind), string(pointsJSON), d.Color, d.Filled, d.StrokeWidth, d.CreatedByParticipantID, d.CreatedAt,
	)
	if err != nil {
		return Drawing{}, err
	}
	return d, nil
}

// GetDrawing loads a single drawing by ID — the WS hub needs its scene
// and author before it can decide whether the caller is allowed to
// erase it.
func (s *Store) GetDrawing(id string) (Drawing, error) {
	var d Drawing
	var kind, pointsJSON string
	err := s.db.QueryRow(
		`SELECT id, scene_id, kind, points, color, filled, stroke_width, created_by_participant_id, created_at
		 FROM drawing WHERE id = ?`, id,
	).Scan(&d.ID, &d.SceneID, &kind, &pointsJSON, &d.Color, &d.Filled, &d.StrokeWidth, &d.CreatedByParticipantID, &d.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Drawing{}, ErrNotFound
		}
		return Drawing{}, err
	}
	d.Kind = DrawingKind(kind)
	if err := json.Unmarshal([]byte(pointsJSON), &d.Points); err != nil {
		return Drawing{}, err
	}
	return d, nil
}

// DeleteDrawing removes a drawing permanently. Deleting one that's
// already gone is not an error — two people can erase the same stroke
// at the same time, and the second one shouldn't fail.
func (s *Store) DeleteDrawing(id string) error {
	_, err := s.db.Exec(`DELETE FROM drawing WHERE id = ?`, id)
	return err
}

// ListDrawingsForScene returns a scene's drawings in creation order, so
// later strokes render on top of earlier ones.
//
// rowid settles a tie on created_at, which is not decoration: that
// column comes from a clock about a millisecond wide, and two people
// drawing at once share it exactly. Left to SQLite, whose stroke was on
// top would then be decided per query — so a reload could swap them, and
// the two clients could disagree. See ListRecentMessages for the long
// version.
func (s *Store) ListDrawingsForScene(sceneID string) ([]Drawing, error) {
	rows, err := s.db.Query(
		`SELECT id, scene_id, kind, points, color, filled, stroke_width, created_by_participant_id, created_at
		 FROM drawing WHERE scene_id = ? ORDER BY created_at ASC, rowid ASC`,
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
		if err := rows.Scan(&d.ID, &d.SceneID, &kind, &pointsJSON, &d.Color, &d.Filled, &d.StrokeWidth, &d.CreatedByParticipantID, &d.CreatedAt); err != nil {
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
