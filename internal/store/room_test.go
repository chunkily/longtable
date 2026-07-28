package store

import (
	"errors"
	"testing"

	"longtable/internal/auth"
)

func TestCreateRoom(t *testing.T) {
	s := newTestStore(t)

	room, gm, err := s.CreateRoom("Curse of Strahd", "Alice", "hunter2")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if room.Slug == "" {
		t.Fatal("expected a generated slug")
	}
	if gm.Role != RoleGM {
		t.Fatalf("founding participant role = %q, want gm", gm.Role)
	}
	if gm.RoomID != room.ID {
		t.Fatalf("participant room ID = %q, want %q", gm.RoomID, room.ID)
	}

	got, err := s.GetRoomBySlug(room.Slug)
	if err != nil {
		t.Fatalf("GetRoomBySlug: %v", err)
	}
	if got.ID != room.ID {
		t.Fatalf("GetRoomBySlug returned room %q, want %q", got.ID, room.ID)
	}
}

func TestCreateRoom_UniqueSlugs(t *testing.T) {
	s := newTestStore(t)

	seen := map[string]bool{}
	for i := 0; i < 10; i++ {
		room, _, err := s.CreateRoom("Room", "GM", "password")
		if err != nil {
			t.Fatalf("CreateRoom: %v", err)
		}
		if seen[room.Slug] {
			t.Fatalf("duplicate slug generated: %q", room.Slug)
		}
		seen[room.Slug] = true
	}
}

func TestGetRoomBySlug_NotFound(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.GetRoomBySlug("nosuch"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestListRooms(t *testing.T) {
	s := newTestStore(t)

	if _, _, err := s.CreateRoom("Room A", "GM", "password"); err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if _, _, err := s.CreateRoom("Room B", "GM", "password"); err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	rooms, err := s.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("len(rooms) = %d, want 2", len(rooms))
	}
}

func TestSetActiveScene(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene 1", nil, 70, 20, 20)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	if err := s.SetActiveScene(room.ID, scene.ID); err != nil {
		t.Fatalf("SetActiveScene: %v", err)
	}

	got, err := s.GetRoomByID(room.ID)
	if err != nil {
		t.Fatalf("GetRoomByID: %v", err)
	}
	if got.ActiveSceneID == nil || *got.ActiveSceneID != scene.ID {
		t.Fatalf("ActiveSceneID = %v, want %q", got.ActiveSceneID, scene.ID)
	}
}

func TestSetGMPassword(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "oldpassword")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	if err := s.SetGMPassword(room.ID, "newpassword"); err != nil {
		t.Fatalf("SetGMPassword: %v", err)
	}

	got, err := s.GetRoomByID(room.ID)
	if err != nil {
		t.Fatalf("GetRoomByID: %v", err)
	}
	if !auth.VerifyPassword(got.GMPasswordHash, "newpassword") {
		t.Fatal("new password does not verify")
	}
	if auth.VerifyPassword(got.GMPasswordHash, "oldpassword") {
		t.Fatal("old password still verifies after change")
	}
}
