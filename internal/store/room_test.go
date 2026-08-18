package store

import (
	"errors"
	"testing"

	"longtable/internal/auth"
)

func TestCreateRoom(t *testing.T) {
	s := newTestStore(t)

	room, gm, err := s.CreateRoom("Curse of Strahd", "Alice", "", "hunter2")
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
		room, _, err := s.CreateRoom("Room", "GM", "", "password")
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

	if _, _, err := s.CreateRoom("Room A", "GM", "", "password"); err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if _, _, err := s.CreateRoom("Room B", "GM", "", "password"); err != nil {
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

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
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

	room, _, err := s.CreateRoom("Room", "GM", "", "oldpassword")
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

// Deleting a room takes everything hanging off it, in one statement,
// through the schema's cascades — and stops short of the shared images,
// which belong to every room that uploaded the same bytes.
func TestDeleteRoom_TakesItsContentsAndLeavesTheImages(t *testing.T) {
	s := newTestStore(t)

	room, gm, err := s.CreateRoom("Finished Campaign", "Alice", "", "hunter2")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Cave", nil, 70, 1000, 800)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	if _, err := s.CreateToken(Token{SceneID: scene.ID, Name: "Goblin"}); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if _, err := s.InsertMessage(Message{
		RoomID: room.ID, ParticipantID: &gm.ID, ParticipantName: "Alice", Body: "hello",
	}); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	// An image this room shares with another one. Only the library
	// membership should go.
	asset, err := s.CreateAsset("shared-hash", "tavern.webp", "image/webp", 2048)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	other, _, err := s.CreateRoom("Still Playing", "Bob", "", "hunter2")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	for _, id := range []string{room.ID, other.ID} {
		if err := s.AddAssetToRoom(id, asset.ID, "Tavern", "", AssetKindMap, nil); err != nil {
			t.Fatalf("AddAssetToRoom: %v", err)
		}
	}

	if err := s.DeleteRoom(room.ID); err != nil {
		t.Fatalf("DeleteRoom: %v", err)
	}

	if _, err := s.GetRoomBySlug(room.Slug); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRoomBySlug after delete: err = %v, want ErrNotFound", err)
	}
	for name, count := range map[string]int{
		"scenes":          countRows(t, s, `SELECT count(*) FROM scene WHERE room_id = ?`, room.ID),
		"tokens":          countRows(t, s, `SELECT count(*) FROM token WHERE scene_id = ?`, scene.ID),
		"messages":        countRows(t, s, `SELECT count(*) FROM message WHERE room_id = ?`, room.ID),
		"participants":    countRows(t, s, `SELECT count(*) FROM participant WHERE room_id = ?`, room.ID),
		"library entries": countRows(t, s, `SELECT count(*) FROM room_asset WHERE room_id = ?`, room.ID),
	} {
		if count != 0 {
			t.Errorf("%s left behind: %d, want 0", name, count)
		}
	}

	// The other room still has the picture, and the asset row is still
	// there for it to point at.
	if _, err := s.GetAsset(asset.ID); err != nil {
		t.Errorf("GetAsset after deleting one of its rooms: %v", err)
	}
	if n := countRows(t, s, `SELECT count(*) FROM room_asset WHERE room_id = ?`, other.ID); n != 1 {
		t.Errorf("the other room's library holds %d assets, want 1", n)
	}
}

func countRows(t *testing.T, s *Store, query, arg string) int {
	t.Helper()

	var n int
	if err := s.db.QueryRow(query, arg).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}
