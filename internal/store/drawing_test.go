package store

import (
	"testing"
	"time"
)

func TestCreateDrawing_ListForScene(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	points := []Point{{X: 1.5, Y: 2.5}, {X: 3, Y: 4}, {X: 5, Y: 6}}
	d, err := s.CreateDrawing(scene.ID, DrawingKindFreehand, points, "#ef4444")
	if err != nil {
		t.Fatalf("CreateDrawing: %v", err)
	}
	if d.ID == "" {
		t.Fatal("expected a generated ID")
	}

	drawings, err := s.ListDrawingsForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	if len(drawings) != 1 {
		t.Fatalf("len(drawings) = %d, want 1", len(drawings))
	}
	got := drawings[0]
	if got.Kind != DrawingKindFreehand {
		t.Fatalf("Kind = %q, want freehand", got.Kind)
	}
	if got.Color != "#ef4444" {
		t.Fatalf("Color = %q, want #ef4444", got.Color)
	}
	if len(got.Points) != len(points) {
		t.Fatalf("len(Points) = %d, want %d", len(got.Points), len(points))
	}
	for i, p := range points {
		if got.Points[i] != p {
			t.Fatalf("Points[%d] = %+v, want %+v", i, got.Points[i], p)
		}
	}
}

func TestListDrawingsForScene_OrderedByCreation(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	first, err := s.CreateDrawing(scene.ID, DrawingKindLine, []Point{{X: 0, Y: 0}, {X: 1, Y: 1}}, "#000000")
	if err != nil {
		t.Fatalf("CreateDrawing: %v", err)
	}

	// created_at has nanosecond precision in the schema, but the clock
	// tick on some platforms is coarser than that; sleep a moment so
	// the two rows sort deterministically by creation time below.
	time.Sleep(2 * time.Millisecond)

	second, err := s.CreateDrawing(scene.ID, DrawingKindRect, []Point{{X: 0, Y: 0}, {X: 2, Y: 2}}, "#ffffff")
	if err != nil {
		t.Fatalf("CreateDrawing: %v", err)
	}

	drawings, err := s.ListDrawingsForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	if len(drawings) != 2 || drawings[0].ID != first.ID || drawings[1].ID != second.ID {
		t.Fatalf("drawings = %+v, want [%q, %q] in order", drawings, first.ID, second.ID)
	}
}

func TestListDrawingsForScene_Empty(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	drawings, err := s.ListDrawingsForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	if len(drawings) != 0 {
		t.Fatalf("len(drawings) = %d, want 0", len(drawings))
	}
}
