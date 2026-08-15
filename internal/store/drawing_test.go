package store

import (
	"errors"
	"testing"
	"time"
)

func TestCreateDrawing_ListForScene(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	points := []Point{{X: 1.5, Y: 2.5}, {X: 3, Y: 4}, {X: 5, Y: 6}}
	d, err := s.CreateDrawing(Drawing{SceneID: scene.ID, Kind: DrawingKindFreehand, Points: points, Color: "#ef4444"})
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
	if got.CreatedByParticipantID != nil {
		t.Fatalf("CreatedByParticipantID = %q, want nil for an unattributed drawing", *got.CreatedByParticipantID)
	}
}

func TestCreateDrawing_RecordsCreator(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	player, err := s.JoinRoom(room.ID, "Bob", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	d, err := s.CreateDrawing(Drawing{SceneID: scene.ID, Kind: DrawingKindLine, Points: []Point{{X: 0, Y: 0}, {X: 1, Y: 1}}, Color: "#cc0000", CreatedByParticipantID: &player.ID})
	if err != nil {
		t.Fatalf("CreateDrawing: %v", err)
	}
	if d.CreatedByParticipantID == nil || *d.CreatedByParticipantID != player.ID {
		t.Fatalf("returned CreatedByParticipantID = %v, want %q", d.CreatedByParticipantID, player.ID)
	}

	drawings, err := s.ListDrawingsForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	if len(drawings) != 1 {
		t.Fatalf("len(drawings) = %d, want 1", len(drawings))
	}
	if got := drawings[0].CreatedByParticipantID; got == nil || *got != player.ID {
		t.Fatalf("loaded CreatedByParticipantID = %v, want %q", got, player.ID)
	}
}

func TestCreateDrawing_RejectsUnknownCreator(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	ghost := "not-a-participant"
	if _, err := s.CreateDrawing(Drawing{SceneID: scene.ID, Kind: DrawingKindLine, Points: []Point{{X: 0, Y: 0}, {X: 1, Y: 1}}, Color: "#cc0000", CreatedByParticipantID: &ghost}); err == nil {
		t.Fatal("expected a foreign key error for a creator that isn't a participant")
	}
}

// A participant row can go away (e.g. its room is being cleaned up)
// while its drawings remain; authorship then reads as unknown rather
// than the drawing disappearing or pointing at a dangling ID.
func TestListDrawingsForScene_CreatorClearedWhenParticipantRemoved(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	player, err := s.JoinRoom(room.ID, "Bob", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	if _, err := s.CreateDrawing(Drawing{SceneID: scene.ID, Kind: DrawingKindRect, Points: []Point{{X: 0, Y: 0}, {X: 2, Y: 2}}, Color: "#cc0000", CreatedByParticipantID: &player.ID}); err != nil {
		t.Fatalf("CreateDrawing: %v", err)
	}

	if _, err := s.db.Exec(`DELETE FROM participant WHERE id = ?`, player.ID); err != nil {
		t.Fatalf("delete participant: %v", err)
	}

	drawings, err := s.ListDrawingsForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	if len(drawings) != 1 {
		t.Fatalf("len(drawings) = %d, want 1 (the drawing must survive its author)", len(drawings))
	}
	if got := drawings[0].CreatedByParticipantID; got != nil {
		t.Fatalf("CreatedByParticipantID = %q, want nil", *got)
	}
}

func TestListDrawingsForScene_OrderedByCreation(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	first, err := s.CreateDrawing(Drawing{SceneID: scene.ID, Kind: DrawingKindLine, Points: []Point{{X: 0, Y: 0}, {X: 1, Y: 1}}, Color: "#000000"})
	if err != nil {
		t.Fatalf("CreateDrawing: %v", err)
	}

	// created_at has nanosecond precision in the schema, but the clock
	// tick on some platforms is coarser than that; sleep a moment so
	// the two rows sort deterministically by creation time below.
	time.Sleep(2 * time.Millisecond)

	second, err := s.CreateDrawing(Drawing{SceneID: scene.ID, Kind: DrawingKindRect, Points: []Point{{X: 0, Y: 0}, {X: 2, Y: 2}}, Color: "#ffffff"})
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

func TestGetDrawing(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	player, err := s.JoinRoom(room.ID, "Bob", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	points := []Point{{X: 1, Y: 2}, {X: 3, Y: 4}}
	created, err := s.CreateDrawing(Drawing{SceneID: scene.ID, Kind: DrawingKindLine, Points: points, Color: "#cc0000", CreatedByParticipantID: &player.ID})
	if err != nil {
		t.Fatalf("CreateDrawing: %v", err)
	}

	got, err := s.GetDrawing(created.ID)
	if err != nil {
		t.Fatalf("GetDrawing: %v", err)
	}
	if got.SceneID != scene.ID {
		t.Fatalf("SceneID = %q, want %q", got.SceneID, scene.ID)
	}
	if got.Kind != DrawingKindLine {
		t.Fatalf("Kind = %q, want line", got.Kind)
	}
	if len(got.Points) != 2 || got.Points[0] != points[0] {
		t.Fatalf("Points = %+v, want %+v", got.Points, points)
	}
	if got.CreatedByParticipantID == nil || *got.CreatedByParticipantID != player.ID {
		t.Fatalf("CreatedByParticipantID = %v, want %q", got.CreatedByParticipantID, player.ID)
	}

	if _, err := s.GetDrawing("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetDrawing(unknown) error = %v, want ErrNotFound", err)
	}
}

func TestDeleteDrawing(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	keep, err := s.CreateDrawing(Drawing{SceneID: scene.ID, Kind: DrawingKindLine, Points: []Point{{X: 0, Y: 0}, {X: 1, Y: 1}}, Color: "#cc0000"})
	if err != nil {
		t.Fatalf("CreateDrawing: %v", err)
	}
	erase, err := s.CreateDrawing(Drawing{SceneID: scene.ID, Kind: DrawingKindRect, Points: []Point{{X: 0, Y: 0}, {X: 2, Y: 2}}, Color: "#008000"})
	if err != nil {
		t.Fatalf("CreateDrawing: %v", err)
	}

	if err := s.DeleteDrawing(erase.ID); err != nil {
		t.Fatalf("DeleteDrawing: %v", err)
	}

	drawings, err := s.ListDrawingsForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	if len(drawings) != 1 || drawings[0].ID != keep.ID {
		t.Fatalf("remaining drawings = %+v, want just %q", drawings, keep.ID)
	}

	// Two people can erase the same stroke at once; the loser of that
	// race must not see a failure.
	if err := s.DeleteDrawing(erase.ID); err != nil {
		t.Fatalf("DeleteDrawing (already deleted): %v", err)
	}
}

func TestListDrawingsForScene_Empty(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
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
