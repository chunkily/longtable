package store

import (
	"errors"
	"testing"
)

func TestCreateScene(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
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

func TestListScenesForRoom_OldestFirstAndScopedToTheRoom(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	other, _, err := s.CreateRoom("Other", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom(other): %v", err)
	}

	for _, name := range []string{"Tavern", "Dungeon"} {
		if _, err := s.CreateScene(room.ID, name, nil, 70, 700, 700); err != nil {
			t.Fatalf("CreateScene(%s): %v", name, err)
		}
	}
	if _, err := s.CreateScene(other.ID, "Theirs", nil, 70, 700, 700); err != nil {
		t.Fatalf("CreateScene(theirs): %v", err)
	}

	scenes, err := s.ListScenesForRoom(room.ID)
	if err != nil {
		t.Fatalf("ListScenesForRoom: %v", err)
	}
	names := make([]string, len(scenes))
	for i, sc := range scenes {
		names[i] = sc.Name
	}
	if len(names) != 2 || names[0] != "Tavern" || names[1] != "Dungeon" {
		t.Fatalf("scenes = %v, want [Tavern Dungeon] and nothing from the other room", names)
	}
}

// The tokens, fog and drawings on a scene go with it via ON DELETE
// CASCADE, which only works because the connection turns foreign keys
// on. If that pragma is ever lost this is the test that catches it.
func TestDeleteScene_TakesItsTokensFogAndDrawingsWithIt(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Doomed", nil, 70, 700, 700)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	survivor, err := s.CreateScene(room.ID, "Survivor", nil, 70, 700, 700)
	if err != nil {
		t.Fatalf("CreateScene(survivor): %v", err)
	}

	for _, sc := range []Scene{scene, survivor} {
		if _, err := s.CreateToken(Token{SceneID: sc.ID, Name: "Goblin", Width: 1, Height: 1}); err != nil {
			t.Fatalf("CreateToken: %v", err)
		}
		if _, err := s.HideCells(sc.ID, []FogCell{{X: 1, Y: 1}}); err != nil {
			t.Fatalf("HideCells: %v", err)
		}
		if _, err := s.CreateDrawing(Drawing{
			SceneID: sc.ID,
			Kind:    DrawingKindLine,
			Points:  []Point{{X: 0, Y: 0}, {X: 1, Y: 1}},
			Color:   "#000000",
		}); err != nil {
			t.Fatalf("CreateDrawing: %v", err)
		}
	}

	if err := s.DeleteScene(scene.ID); err != nil {
		t.Fatalf("DeleteScene: %v", err)
	}

	if _, err := s.GetScene(scene.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetScene err = %v, want ErrNotFound", err)
	}
	if tokens, err := s.ListTokensForScene(scene.ID); err != nil || len(tokens) != 0 {
		t.Fatalf("tokens = %+v (err %v), want none left", tokens, err)
	}
	if chunks, err := s.ListFogChunks(scene.ID); err != nil || len(chunks) != 0 {
		t.Fatalf("fog chunks = %+v (err %v), want none left", chunks, err)
	}
	if drawings, err := s.ListDrawingsForScene(scene.ID); err != nil || len(drawings) != 0 {
		t.Fatalf("drawings = %+v (err %v), want none left", drawings, err)
	}

	// The other scene in the same room keeps everything.
	if tokens, err := s.ListTokensForScene(survivor.ID); err != nil || len(tokens) != 1 {
		t.Fatalf("survivor tokens = %+v (err %v), want 1", tokens, err)
	}
	if chunks, err := s.ListFogChunks(survivor.ID); err != nil || len(chunks) != 1 {
		t.Fatalf("survivor fog = %+v (err %v), want 1", chunks, err)
	}
	if drawings, err := s.ListDrawingsForScene(survivor.ID); err != nil || len(drawings) != 1 {
		t.Fatalf("survivor drawings = %+v (err %v), want 1", drawings, err)
	}
}

func TestSetSceneMap_SwapsTheImageAndBoundsButKeepsWhatIsOnIt(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 700, 700)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	token, err := s.CreateToken(Token{SceneID: scene.ID, Name: "Goblin", X: 2, Y: 3, Width: 1, Height: 1})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if _, err := s.HideCells(scene.ID, []FogCell{{X: 1, Y: 1}}); err != nil {
		t.Fatalf("HideCells: %v", err)
	}

	asset, err := s.CreateAsset("hash", "map.webp", "image/webp", 10)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if err := s.SetSceneMap(scene.ID, &asset.ID, 1400, 1000); err != nil {
		t.Fatalf("SetSceneMap: %v", err)
	}

	got, err := s.GetScene(scene.ID)
	if err != nil {
		t.Fatalf("GetScene: %v", err)
	}
	if got.MapAssetID == nil || *got.MapAssetID != asset.ID {
		t.Fatalf("map asset = %v, want %q", got.MapAssetID, asset.ID)
	}
	if got.Width != 1400 || got.Height != 1000 {
		t.Fatalf("bounds = %dx%d, want 1400x1000", got.Width, got.Height)
	}

	// The whole point of replacing rather than recreating: progress stays.
	tokens, err := s.ListTokensForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListTokensForScene: %v", err)
	}
	if len(tokens) != 1 || tokens[0].ID != token.ID || tokens[0].X != 2 || tokens[0].Y != 3 {
		t.Fatalf("tokens = %+v, want the original still at (2,3)", tokens)
	}
	if chunks, err := s.ListFogChunks(scene.ID); err != nil || len(chunks) != 1 {
		t.Fatalf("fog chunks = %+v (err %v), want the hidden cell kept", chunks, err)
	}
}

func TestCreateToken_DefaultsAndLookup(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
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

// Every read path pads to TrackerSlots, so callers can index slots 0..2
// without checking the length — including for a token created before the
// columns existed, whose stored value is the empty JSON array the
// migration defaulted it to. Creating one with no trackers at all
// produces exactly that shape, which is why this stands in for the
// migration too.
func TestToken_TrackersAreAlwaysThreeSlotsOnTheWayOut(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	bare, err := s.CreateToken(Token{SceneID: scene.ID, Name: "Goblin"})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	loaded, err := s.GetToken(bare.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if len(loaded.Trackers) != TrackerSlots {
		t.Fatalf("trackers = %+v, want %d empty slots", loaded.Trackers, TrackerSlots)
	}
	if loaded.Conditions == nil {
		t.Fatalf("conditions = nil, want an empty list — a nil one marshals to JSON null")
	}

	// A single slot supplied is padded rather than kept short.
	one, err := s.CreateToken(Token{
		SceneID: scene.ID, Name: "Ogre",
		Trackers: []Tracker{{Label: "HP", Value: func() *int { v := 0; return &v }()}},
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	tokens, err := s.ListTokensForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListTokensForScene: %v", err)
	}
	for _, tk := range tokens {
		if len(tk.Trackers) != TrackerSlots {
			t.Fatalf("token %q trackers = %+v, want %d slots from the list query too", tk.Name, tk.Trackers, TrackerSlots)
		}
		// The one that matters: a creature on nought hit points is a set
		// value, not an empty slot, and a round trip through JSON and
		// SQLite is where the two would collapse into each other.
		if tk.ID == one.ID && (tk.Trackers[0].Value == nil || *tk.Trackers[0].Value != 0) {
			t.Fatalf("HP = %+v, want a stored 0", tk.Trackers[0])
		}
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

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
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
