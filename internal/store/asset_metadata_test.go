package store

import (
	"errors"
	"testing"
)

// The per-room half of a library entry: what this room calls the asset,
// how it credits it, and what it measured the map's squares at. All
// three sit on room_asset rather than the shared asset row, so two rooms
// holding the same file can disagree about every one of them.

func TestRoomLibrary_NamesAndGridSizesAreScopedPerRoom(t *testing.T) {
	s := newTestStore(t)

	roomA, _, err := s.CreateRoom("Room A", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	roomB, _, err := s.CreateRoom("Room B", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	asset, err := s.CreateAsset("shared", "map.webp", "image/webp", 100)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	seventy, sixtyfour := 70, 64
	if err := s.AddAssetToRoom(roomA.ID, asset.ID, "Sunless citadel", "by Alice", &seventy); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}
	if err := s.AddAssetToRoom(roomB.ID, asset.ID, "The pit", "found online", &sixtyfour); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	inA := onlyLibraryEntry(t, s, roomA.ID)
	if inA.Name != "Sunless citadel" || *inA.GridSize != 70 {
		t.Fatalf("room A entry = %q at %d px", inA.Name, *inA.GridSize)
	}
	inB := onlyLibraryEntry(t, s, roomB.ID)
	if inB.Name != "The pit" || *inB.GridSize != 64 {
		t.Fatalf("room B entry = %q at %d px", inB.Name, *inB.GridSize)
	}
}

// Adding a file the room already has is how people discover it was
// already there. It mustn't wipe details the earlier add recorded and
// this one simply didn't carry — which for a token upload is every one
// of them.
func TestAddAssetToRoom_KeepsDetailsWhenNoneGiven(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	asset, err := s.CreateAsset("hash", "a.webp", "image/webp", 1)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	seventy := 70
	if err := s.AddAssetToRoom(room.ID, asset.ID, "Ruined keep", "by Bob", &seventy); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}
	if err := s.AddAssetToRoom(room.ID, asset.ID, "", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	entry := onlyLibraryEntry(t, s, room.ID)
	if entry.Name != "Ruined keep" {
		t.Fatalf("name = %q, want it kept", entry.Name)
	}
	if entry.GridSize == nil || *entry.GridSize != 70 {
		t.Fatalf("gridSize = %v, want it kept at 70", entry.GridSize)
	}

	// A real correction still lands.
	sixty := 60
	if err := s.AddAssetToRoom(room.ID, asset.ID, "The keep", "", &sixty); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}
	entry = onlyLibraryEntry(t, s, room.ID)
	if entry.Name != "The keep" || *entry.GridSize != 60 {
		t.Fatalf("entry = %q at %d px, want the correction applied", entry.Name, *entry.GridSize)
	}
}

// Everything added before names existed has an empty one, and an entry
// with no name is invisible in a library you search by name.
func TestListRoomAssets_FallsBackToTheFilenameWhenUnnamed(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	asset, err := s.CreateAsset("hash", "old goblin.webp", "image/webp", 1)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if err := s.AddAssetToRoom(room.ID, asset.ID, "", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	if got := onlyLibraryEntry(t, s, room.ID).Name; got != "old goblin" {
		t.Fatalf("name = %q, want the filename without its extension", got)
	}
}

func TestUpdateRoomAsset(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	other, _, err := s.CreateRoom("Other", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	asset, err := s.CreateAsset("hash", "img_4471.webp", "image/webp", 1)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	seventy := 70
	if err := s.AddAssetToRoom(room.ID, asset.ID, "img_4471", "by Bob", &seventy); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	if err := s.UpdateRoomAsset(room.ID, asset.ID, "Ruined keep", ""); err != nil {
		t.Fatalf("UpdateRoomAsset: %v", err)
	}
	entry := onlyLibraryEntry(t, s, room.ID)
	if entry.Name != "Ruined keep" {
		t.Fatalf("name = %q, want it renamed", entry.Name)
	}
	// Editing a form is the one place an empty credit means "clear it".
	if entry.Attribution != "" {
		t.Fatalf("attribution = %q, want it cleared", entry.Attribution)
	}
	// The measured grid isn't the caller's to change — it describes the
	// stored pixels, and those didn't move.
	if entry.GridSize == nil || *entry.GridSize != 70 {
		t.Fatalf("gridSize = %v, want it left at 70", entry.GridSize)
	}

	// A room that doesn't hold the asset gets the same answer as one
	// asking about an asset that doesn't exist.
	if err := s.UpdateRoomAsset(other.ID, asset.ID, "Mine now", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign room: err = %v, want ErrNotFound", err)
	}
	if err := s.UpdateRoomAsset(room.ID, "no-such-asset", "Ghost", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing asset: err = %v, want ErrNotFound", err)
	}
}

func TestDisplayNameFromFilename(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"goblin archer.webp", "goblin archer"},
		{"dungeon_final_v2.png", "dungeon_final_v2"},
		{"no-extension", "no-extension"},
		{"maps/tavern.webp", "tavern"},
		{`C:\art\owlbear.webp`, "owlbear"},
		// A dotfile is all extension and no name; something has to show in
		// the grid.
		{".webp", "Untitled"},
		{"", "Untitled"},
	} {
		if got := DisplayNameFromFilename(tc.in); got != tc.want {
			t.Errorf("DisplayNameFromFilename(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func onlyLibraryEntry(t *testing.T, s *Store, roomID string) LibraryAsset {
	t.Helper()

	library, err := s.ListRoomAssets(roomID)
	if err != nil {
		t.Fatalf("ListRoomAssets: %v", err)
	}
	if len(library) != 1 {
		t.Fatalf("library has %d entries, want 1", len(library))
	}
	return library[0]
}
