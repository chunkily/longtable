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
	if err := s.AddAssetToRoom(roomA.ID, asset.ID, "Sunless citadel", "by Alice", AssetKindMap, &seventy); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}
	if err := s.AddAssetToRoom(roomB.ID, asset.ID, "The pit", "found online", AssetKindMap, &sixtyfour); err != nil {
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

// The kind is per-room too, and it's the one field where that isn't
// merely defensible: the same picture genuinely is a portrait to one
// group and the map they fight on to another.
func TestRoomLibrary_KindIsScopedPerRoom(t *testing.T) {
	s := newTestStore(t)

	roomA, _, err := s.CreateRoom("Room A", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	roomB, _, err := s.CreateRoom("Room B", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	asset, err := s.CreateAsset("shared", "dragon.webp", "image/webp", 100)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	if err := s.AddAssetToRoom(roomA.ID, asset.ID, "Ancient red", "", AssetKindToken, nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}
	if err := s.AddAssetToRoom(roomB.ID, asset.ID, "Dragon mural", "", AssetKindMap, nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	if got := onlyLibraryEntry(t, s, roomA.ID).Kind; got != AssetKindToken {
		t.Errorf("room A kind = %q, want token", got)
	}
	if got := onlyLibraryEntry(t, s, roomB.ID).Kind; got != AssetKindMap {
		t.Errorf("room B kind = %q, want map", got)
	}
}

// An upload that says nothing about the kind is a token, because that's
// what most art is and because the alternative — refusing it — would
// break every client that predates the split.
func TestAddAssetToRoom_DefaultsToToken(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	asset, err := s.CreateAsset("hash", "a.webp", "image/webp", 1)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if err := s.AddAssetToRoom(room.ID, asset.ID, "", "", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	if got := onlyLibraryEntry(t, s, room.ID).Kind; got != AssetKindToken {
		t.Fatalf("kind = %q, want token", got)
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
	if err := s.AddAssetToRoom(room.ID, asset.ID, "Ruined keep", "by Bob", AssetKindMap, &seventy); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}
	if err := s.AddAssetToRoom(room.ID, asset.ID, "", "", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	entry := onlyLibraryEntry(t, s, room.ID)
	if entry.Name != "Ruined keep" {
		t.Fatalf("name = %q, want it kept", entry.Name)
	}
	if entry.GridSize == nil || *entry.GridSize != 70 {
		t.Fatalf("gridSize = %v, want it kept at 70", entry.GridSize)
	}
	// The kind has a column default, so a silent re-add is the one way it
	// could quietly become a token again — and a map that files itself
	// under Tokens on every re-upload is the bug this guards.
	if entry.Kind != AssetKindMap {
		t.Fatalf("kind = %q, want it kept as a map", entry.Kind)
	}

	// A real correction still lands.
	sixty := 60
	if err := s.AddAssetToRoom(room.ID, asset.ID, "The keep", "", AssetKindToken, &sixty); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}
	entry = onlyLibraryEntry(t, s, room.ID)
	if entry.Name != "The keep" || *entry.GridSize != 60 || entry.Kind != AssetKindToken {
		t.Fatalf("entry = %q at %d px, kind %q, want the correction applied",
			entry.Name, *entry.GridSize, entry.Kind)
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
	if err := s.AddAssetToRoom(room.ID, asset.ID, "", "", "", nil); err != nil {
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
	if err := s.AddAssetToRoom(room.ID, asset.ID, "img_4471", "by Bob", AssetKindMap, &seventy); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	if err := s.UpdateRoomAsset(room.ID, asset.ID, "Ruined keep", "", AssetKindMap); err != nil {
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
	if err := s.UpdateRoomAsset(other.ID, asset.ID, "Mine now", "", AssetKindMap); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign room: err = %v, want ErrNotFound", err)
	}
	if err := s.UpdateRoomAsset(room.ID, "no-such-asset", "Ghost", "", AssetKindMap); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing asset: err = %v, want ErrNotFound", err)
	}
}

// Reclassifying is the escape hatch for everything the kind can't be
// sure of: the migration's guess about old rows, and anyone who picked
// the wrong one on the way in.
func TestUpdateRoomAsset_MovesAnAssetBetweenKinds(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	asset, err := s.CreateAsset("hash", "tavern.webp", "image/webp", 1)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if err := s.AddAssetToRoom(room.ID, asset.ID, "Tavern", "", AssetKindToken, nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	if err := s.UpdateRoomAsset(room.ID, asset.ID, "Tavern", "", AssetKindMap); err != nil {
		t.Fatalf("UpdateRoomAsset: %v", err)
	}
	if got := onlyLibraryEntry(t, s, room.ID).Kind; got != AssetKindMap {
		t.Fatalf("kind = %q, want map", got)
	}

	// An edit that doesn't mention the kind leaves it alone, since there
	// is no third kind for a blank to mean.
	if err := s.UpdateRoomAsset(room.ID, asset.ID, "Tavern", "by Bob", ""); err != nil {
		t.Fatalf("UpdateRoomAsset: %v", err)
	}
	if got := onlyLibraryEntry(t, s, room.ID).Kind; got != AssetKindMap {
		t.Fatalf("kind = %q after an edit that omitted it, want map", got)
	}
}

// Removing an asset is about one room's shelf. The file is global and
// content-addressed, so it has to survive for every other room that
// added the same picture — and for this room, if it adds it again.
func TestRemoveAssetFromRoom_LeavesTheFileAndOtherRooms(t *testing.T) {
	s := newTestStore(t)

	roomA, _, err := s.CreateRoom("Room A", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	roomB, _, err := s.CreateRoom("Room B", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	asset, err := s.CreateAsset("shared", "keep.webp", "image/webp", 1)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if err := s.AddAssetToRoom(roomA.ID, asset.ID, "Keep", "by Alice", AssetKindMap, nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}
	if err := s.AddAssetToRoom(roomB.ID, asset.ID, "The pit", "", AssetKindMap, nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	if err := s.RemoveAssetFromRoom(roomA.ID, asset.ID); err != nil {
		t.Fatalf("RemoveAssetFromRoom: %v", err)
	}

	library, err := s.ListRoomAssets(roomA.ID)
	if err != nil {
		t.Fatalf("ListRoomAssets: %v", err)
	}
	if len(library) != 0 {
		t.Fatalf("room A library has %d entries, want 0", len(library))
	}
	// The other room's copy, its own name and credit, and the asset row
	// itself are all untouched.
	if got := onlyLibraryEntry(t, s, roomB.ID).Name; got != "The pit" {
		t.Errorf("room B entry = %q, want it untouched", got)
	}
	if _, err := s.GetAsset(asset.ID); err != nil {
		t.Errorf("GetAsset after removal: %v, want the asset row to survive", err)
	}

	// Adding it back is the undo, and it comes back blank rather than
	// carrying the name and credit the removed row had — the row is gone,
	// not archived.
	if err := s.AddAssetToRoom(roomA.ID, asset.ID, "Keep again", "", AssetKindMap, nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}
	entry := onlyLibraryEntry(t, s, roomA.ID)
	if entry.Name != "Keep again" || entry.Attribution != "" {
		t.Errorf("re-added entry = %q credited %q, want a fresh row", entry.Name, entry.Attribution)
	}
}

// Another room's asset and an asset that doesn't exist give the same
// answer, so a removal can't be used to probe what exists elsewhere.
func TestRemoveAssetFromRoom_WontReachIntoAnotherRoomsLibrary(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	other, _, err := s.CreateRoom("Other", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	asset, err := s.CreateAsset("hash", "a.webp", "image/webp", 1)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if err := s.AddAssetToRoom(room.ID, asset.ID, "Mine", "", AssetKindToken, nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	if err := s.RemoveAssetFromRoom(other.ID, asset.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("foreign room: err = %v, want ErrNotFound", err)
	}
	if err := s.RemoveAssetFromRoom(room.ID, "no-such-asset"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing asset: err = %v, want ErrNotFound", err)
	}
	if got := onlyLibraryEntry(t, s, room.ID).Name; got != "Mine" {
		t.Errorf("entry = %q, want it still there", got)
	}
}

// A library that predates the token/map split still has to land in the
// right tab, and whether someone aligned the image is the only signal an
// old row carries. Dropping the column from a live database and
// migrating again is as close to opening an old file as a test gets.
func TestMigrate_SortsOldAssetsByWhetherTheyWereAligned(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	aligned, err := s.CreateAsset("map-hash", "keep.webp", "image/webp", 1)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	plain, err := s.CreateAsset("token-hash", "goblin.webp", "image/webp", 1)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	seventy := 70
	if err := s.AddAssetToRoom(room.ID, aligned.ID, "Keep", "", "", &seventy); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}
	if err := s.AddAssetToRoom(room.ID, plain.ID, "Goblin", "", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	if _, err := s.db.Exec(`ALTER TABLE room_asset DROP COLUMN kind`); err != nil {
		t.Fatalf("drop kind column: %v", err)
	}
	if err := s.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if got := kindOf(t, s, room.ID, aligned.ID); got != AssetKindMap {
		t.Errorf("aligned asset = %q, want map", got)
	}
	if got := kindOf(t, s, room.ID, plain.ID); got != AssetKindToken {
		t.Errorf("unaligned asset = %q, want token", got)
	}

	// Once someone corrects the guess, a later boot must not guess again —
	// which is why the backfill is tied to adding the column rather than
	// to every migrate().
	if err := s.UpdateRoomAsset(room.ID, aligned.ID, "Keep", "", AssetKindToken); err != nil {
		t.Fatalf("UpdateRoomAsset: %v", err)
	}
	if err := s.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if got := kindOf(t, s, room.ID, aligned.ID); got != AssetKindToken {
		t.Errorf("corrected asset = %q after a second migrate, want token", got)
	}
}

func kindOf(t *testing.T, s *Store, roomID, assetID string) AssetKind {
	t.Helper()

	entry, err := s.GetRoomAsset(roomID, assetID)
	if err != nil {
		t.Fatalf("GetRoomAsset: %v", err)
	}
	return entry.Kind
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
