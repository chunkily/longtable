package store

import (
	"errors"
	"testing"
)

func TestCreateScene(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	scene, err := s.CreateScene(room.ID, "Battle Map", nil, 70, 30, 20)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	if scene.RoomID != room.ID {
		t.Fatalf("scene room ID = %q, want %q", scene.RoomID, room.ID)
	}

	roomID, err := s.SceneRoomID(scene.ID)
	if err != nil {
		t.Fatalf("SceneRoomID: %v", err)
	}
	if roomID != room.ID {
		t.Fatalf("SceneRoomID = %q, want %q", roomID, room.ID)
	}
}

func TestSceneRoomID_NotFound(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.SceneRoomID("nosuch"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestCreateToken_DefaultsAndLookup(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	token, err := s.CreateToken(Token{SceneID: scene.ID, Name: "Goblin", X: 1, Y: 2, Width: 1, Height: 1})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if token.Visibility != VisibilityVisible {
		t.Fatalf("visibility = %q, want visible default", token.Visibility)
	}

	roomID, err := s.TokenRoomID(token.ID)
	if err != nil {
		t.Fatalf("TokenRoomID: %v", err)
	}
	if roomID != room.ID {
		t.Fatalf("TokenRoomID = %q, want %q", roomID, room.ID)
	}

	tokens, err := s.ListTokensForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListTokensForScene: %v", err)
	}
	if len(tokens) != 1 || tokens[0].ID != token.ID {
		t.Fatalf("ListTokensForScene = %+v, want single token %q", tokens, token.ID)
	}
}

func TestTokenRoomID_NotFound(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.TokenRoomID("nosuch"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestMoveToken(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	token, err := s.CreateToken(Token{SceneID: scene.ID, Name: "Goblin"})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	if err := s.MoveToken(token.ID, 5, 6); err != nil {
		t.Fatalf("MoveToken: %v", err)
	}

	tokens, err := s.ListTokensForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListTokensForScene: %v", err)
	}
	if tokens[0].X != 5 || tokens[0].Y != 6 {
		t.Fatalf("token position = (%v, %v), want (5, 6)", tokens[0].X, tokens[0].Y)
	}
}

func TestRevealCells_Idempotent(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	cells := []FogCell{{X: 1, Y: 1}, {X: 2, Y: 2}}
	if err := s.RevealCells(scene.ID, cells); err != nil {
		t.Fatalf("RevealCells: %v", err)
	}
	if err := s.RevealCells(scene.ID, cells); err != nil {
		t.Fatalf("RevealCells (repeat): %v", err)
	}

	got, err := s.ListFogCells(scene.ID)
	if err != nil {
		t.Fatalf("ListFogCells: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(cells) = %d, want 2 (revealing twice must not duplicate)", len(got))
	}
}

// twoScenesWithFog gives two scenes in one room, both with the same
// cells revealed — the setup that catches a hide or clear that forgets
// to scope itself, since fog coordinates repeat across every scene.
func twoScenesWithFog(t *testing.T) (*Store, Scene, Scene) {
	t.Helper()

	s := newTestStore(t)
	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	cells := []FogCell{{X: 1, Y: 1}, {X: 2, Y: 2}}
	scenes := make([]Scene, 2)
	for i, name := range []string{"First", "Second"} {
		sc, err := s.CreateScene(room.ID, name, nil, 70, 700, 700)
		if err != nil {
			t.Fatalf("CreateScene(%s): %v", name, err)
		}
		if err := s.RevealCells(sc.ID, cells); err != nil {
			t.Fatalf("RevealCells(%s): %v", name, err)
		}
		scenes[i] = sc
	}
	return s, scenes[0], scenes[1]
}

func TestHideCells_RemovesOnlyTheNamedCellsInThatScene(t *testing.T) {
	s, scene, other := twoScenesWithFog(t)

	if err := s.HideCells(scene.ID, []FogCell{{X: 1, Y: 1}}); err != nil {
		t.Fatalf("HideCells: %v", err)
	}

	got, err := s.ListFogCells(scene.ID)
	if err != nil {
		t.Fatalf("ListFogCells: %v", err)
	}
	if len(got) != 1 || got[0] != (FogCell{X: 2, Y: 2}) {
		t.Fatalf("cells = %+v, want only (2,2) left", got)
	}

	// The other scene has a (1,1) of its own, which this hide must not reach.
	otherCells, err := s.ListFogCells(other.ID)
	if err != nil {
		t.Fatalf("ListFogCells(other): %v", err)
	}
	if len(otherCells) != 2 {
		t.Fatalf("other scene cells = %+v, want both still revealed", otherCells)
	}
}

func TestHideCells_UnrevealedCellIsANoOp(t *testing.T) {
	s, scene, _ := twoScenesWithFog(t)

	// Hiding what was never revealed is how an eraser-style sweep behaves
	// at the edges of the painted area, so it has to be silent rather than
	// an error.
	if err := s.HideCells(scene.ID, []FogCell{{X: 9, Y: 9}}); err != nil {
		t.Fatalf("HideCells: %v", err)
	}

	got, err := s.ListFogCells(scene.ID)
	if err != nil {
		t.Fatalf("ListFogCells: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(cells) = %d, want 2 untouched", len(got))
	}
}

func TestClearFog_EmptiesOneSceneOnly(t *testing.T) {
	s, scene, other := twoScenesWithFog(t)

	if err := s.ClearFog(scene.ID); err != nil {
		t.Fatalf("ClearFog: %v", err)
	}

	got, err := s.ListFogCells(scene.ID)
	if err != nil {
		t.Fatalf("ListFogCells: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("cells = %+v, want none", got)
	}

	otherCells, err := s.ListFogCells(other.ID)
	if err != nil {
		t.Fatalf("ListFogCells(other): %v", err)
	}
	if len(otherCells) != 2 {
		t.Fatalf("other scene cells = %+v, want both still revealed", otherCells)
	}
}
