package store

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestCreateAsset_FindByHash(t *testing.T) {
	s := newTestStore(t)

	asset, err := s.CreateAsset("abc123", "map.png", "image/png", 1024)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	got, err := s.FindAssetByHash("abc123")
	if err != nil {
		t.Fatalf("FindAssetByHash: %v", err)
	}
	if got.ID != asset.ID {
		t.Fatalf("resolved asset %q, want %q", got.ID, asset.ID)
	}

	got2, err := s.GetAsset(asset.ID)
	if err != nil {
		t.Fatalf("GetAsset: %v", err)
	}
	if got2.ContentHash != "abc123" {
		t.Fatalf("ContentHash = %q, want abc123", got2.ContentHash)
	}
}

func TestFindAssetByHash_NotFound(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.FindAssetByHash("nosuch"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestCreateAsset_DuplicateHashRejected(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.CreateAsset("dup", "a.png", "image/png", 1); err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	_, err := s.CreateAsset("dup", "b.png", "image/png", 2)
	if err == nil {
		t.Fatal("expected an error inserting a duplicate content hash")
	}
	if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
		t.Fatalf("err = %v, want a unique constraint error", err)
	}
}

// --- room libraries ---
//
// The model these exercise: asset rows are global and content-addressed
// so identical uploads share one stored file, while room_asset decides
// what each room can see. Getting that split wrong in either direction is
// a real bug — duplicated storage on one side, one room's art leaking
// into another on the other.

func TestRoomLibrary_ScopedPerRoomOverOneSharedAsset(t *testing.T) {
	s := newTestStore(t)

	roomA, _, err := s.CreateRoom("Room A", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	roomB, _, err := s.CreateRoom("Room B", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	asset, err := s.CreateAsset("shared-hash", "tavern.webp", "image/webp", 2048)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	if err := s.AddAssetToRoom(roomA.ID, asset.ID, "", "by Alice, CC-BY", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	inA, err := s.ListRoomAssets(roomA.ID)
	if err != nil {
		t.Fatalf("ListRoomAssets: %v", err)
	}
	if len(inA) != 1 || inA[0].ID != asset.ID {
		t.Fatalf("room A library = %+v, want the one asset", inA)
	}
	if inA[0].Attribution != "by Alice, CC-BY" {
		t.Fatalf("attribution = %q, want %q", inA[0].Attribution, "by Alice, CC-BY")
	}

	// The asset exists and room B could name its ID, but it isn't theirs
	// to use until they add it themselves.
	inB, err := s.ListRoomAssets(roomB.ID)
	if err != nil {
		t.Fatalf("ListRoomAssets: %v", err)
	}
	if len(inB) != 0 {
		t.Fatalf("room B library = %+v, want empty", inB)
	}

	for _, tc := range []struct {
		roomID string
		want   bool
		name   string
	}{
		{roomA.ID, true, "room A"},
		{roomB.ID, false, "room B"},
	} {
		got, err := s.AssetInRoom(tc.roomID, asset.ID)
		if err != nil {
			t.Fatalf("AssetInRoom: %v", err)
		}
		if got != tc.want {
			t.Fatalf("AssetInRoom(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}

	// Room B adding the same file shares the asset row rather than
	// duplicating it — that's the whole point of hashing content.
	if err := s.AddAssetToRoom(roomB.ID, asset.ID, "", "found on the internet", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}
	inB, err = s.ListRoomAssets(roomB.ID)
	if err != nil {
		t.Fatalf("ListRoomAssets: %v", err)
	}
	if len(inB) != 1 || inB[0].ID != asset.ID {
		t.Fatalf("room B library = %+v, want the shared asset", inB)
	}
	// Each room credits its own copy: B's note must not have overwritten A's.
	if inB[0].Attribution != "found on the internet" {
		t.Fatalf("room B attribution = %q", inB[0].Attribution)
	}
	inA, err = s.ListRoomAssets(roomA.ID)
	if err != nil {
		t.Fatalf("ListRoomAssets: %v", err)
	}
	if inA[0].Attribution != "by Alice, CC-BY" {
		t.Fatalf("room A attribution became %q", inA[0].Attribution)
	}
}

// Re-uploading a file the room already has is how people discover it was
// already there, so it has to be a no-op rather than an error or a
// duplicate row.
func TestAddAssetToRoom_IsIdempotent(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	asset, err := s.CreateAsset("hash", "a.webp", "image/webp", 1)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	if err := s.AddAssetToRoom(room.ID, asset.ID, "", "original credit", "", nil); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if err := s.AddAssetToRoom(room.ID, asset.ID, "", "original credit", "", nil); err != nil {
		t.Fatalf("second add: %v", err)
	}

	library, err := s.ListRoomAssets(room.ID)
	if err != nil {
		t.Fatalf("ListRoomAssets: %v", err)
	}
	if len(library) != 1 {
		t.Fatalf("library has %d entries, want 1", len(library))
	}
}

// A re-upload with no attribution shouldn't wipe the credit someone
// already recorded — the second uploader simply didn't have anything to
// add, which is not the same as asking for it to be cleared.
func TestAddAssetToRoom_KeepsExistingAttributionWhenNoneGiven(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	asset, err := s.CreateAsset("hash", "a.webp", "image/webp", 1)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	if err := s.AddAssetToRoom(room.ID, asset.ID, "", "art by Bob", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}
	if err := s.AddAssetToRoom(room.ID, asset.ID, "", "", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	library, err := s.ListRoomAssets(room.ID)
	if err != nil {
		t.Fatalf("ListRoomAssets: %v", err)
	}
	if library[0].Attribution != "art by Bob" {
		t.Fatalf("attribution = %q, want it kept as %q", library[0].Attribution, "art by Bob")
	}

	// But a real correction does land.
	if err := s.AddAssetToRoom(room.ID, asset.ID, "", "actually by Carol", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}
	library, err = s.ListRoomAssets(room.ID)
	if err != nil {
		t.Fatalf("ListRoomAssets: %v", err)
	}
	if library[0].Attribution != "actually by Carol" {
		t.Fatalf("attribution = %q, want it updated", library[0].Attribution)
	}
}

func TestListRoomAssets_NewestFirst(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	var ids []string
	for i, hash := range []string{"h1", "h2", "h3"} {
		asset, err := s.CreateAsset(hash, "a.webp", "image/webp", int64(i+1))
		if err != nil {
			t.Fatalf("CreateAsset: %v", err)
		}
		if err := s.AddAssetToRoom(room.ID, asset.ID, "", "", "", nil); err != nil {
			t.Fatalf("AddAssetToRoom: %v", err)
		}
		ids = append(ids, asset.ID)
	}

	library, err := s.ListRoomAssets(room.ID)
	if err != nil {
		t.Fatalf("ListRoomAssets: %v", err)
	}
	// The whole order, not just the head: three assets added in one pass
	// share an added_at exactly — time.Now() is about millisecond-grained
	// on Windows — so this is the case that used to be decided by a random
	// tie-break and fail three runs in ten. Deliberately no sleep between
	// the adds above: adding several files in one go is what the assets
	// page does, and pacing the test would test something nobody does.
	got := make([]string, len(library))
	for i, la := range library {
		got[i] = la.ID
	}
	want := []string{ids[2], ids[1], ids[0]}
	if !slices.Equal(got, want) {
		t.Fatalf("library order = %v, want newest first %v", got, want)
	}
}

// Removing an entry and adding another must still leave the new one on
// top. Worth its own test because the ordering is by rowid, and SQLite
// reuses the rowid freed by deleting the newest row — the reuse is
// always above everything still there, which is the property this
// depends on.
func TestListRoomAssets_NewestFirstAfterARemoval(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	var ids []string
	for i, hash := range []string{"h1", "h2", "h3"} {
		asset, err := s.CreateAsset(hash, "a.webp", "image/webp", int64(i+1))
		if err != nil {
			t.Fatalf("CreateAsset: %v", err)
		}
		if err := s.AddAssetToRoom(room.ID, asset.ID, "", "", "", nil); err != nil {
			t.Fatalf("AddAssetToRoom: %v", err)
		}
		ids = append(ids, asset.ID)
	}

	// Take the newest back out, which is what frees its rowid.
	if err := s.RemoveAssetFromRoom(room.ID, ids[2]); err != nil {
		t.Fatalf("RemoveAssetFromRoom: %v", err)
	}

	fourth, err := s.CreateAsset("h4", "a.webp", "image/webp", 4)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if err := s.AddAssetToRoom(room.ID, fourth.ID, "", "", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	library, err := s.ListRoomAssets(room.ID)
	if err != nil {
		t.Fatalf("ListRoomAssets: %v", err)
	}
	got := make([]string, len(library))
	for i, la := range library {
		got[i] = la.ID
	}
	want := []string{fourth.ID, ids[1], ids[0]}
	if !slices.Equal(got, want) {
		t.Fatalf("library order = %v, want the re-added one first %v", got, want)
	}
}

// Re-adding bytes a room already has updates the entry in place rather
// than promoting it: the upsert doesn't restamp when it landed, and the
// rowid it is ordered by doesn't move on an update either. Two halves of
// one answer, and this is what fails if they ever stop agreeing.
func TestListRoomAssets_ReAddingKeepsAnEntryWhereItWas(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	var ids []string
	for i, hash := range []string{"h1", "h2"} {
		asset, err := s.CreateAsset(hash, "a.webp", "image/webp", int64(i+1))
		if err != nil {
			t.Fatalf("CreateAsset: %v", err)
		}
		if err := s.AddAssetToRoom(room.ID, asset.ID, "", "", "", nil); err != nil {
			t.Fatalf("AddAssetToRoom: %v", err)
		}
		ids = append(ids, asset.ID)
	}

	// The older of the two, added again under a new name.
	if err := s.AddAssetToRoom(room.ID, ids[0], "Renamed", "", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom (again): %v", err)
	}

	library, err := s.ListRoomAssets(room.ID)
	if err != nil {
		t.Fatalf("ListRoomAssets: %v", err)
	}
	if len(library) != 2 {
		t.Fatalf("library has %d entries, want 2 — re-adding must not make a second", len(library))
	}
	if library[0].ID != ids[1] {
		t.Fatal("re-adding an entry moved it to the top; it should stay where it was")
	}
	if library[1].Name != "Renamed" {
		t.Fatalf("name = %q, want the re-add to have renamed it", library[1].Name)
	}
}
