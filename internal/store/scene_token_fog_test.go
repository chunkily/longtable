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
