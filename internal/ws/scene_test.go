package ws

import (
	"encoding/json"
	"testing"
	"time"

	"longtable/internal/store"
)

// sceneTestRoom is a room with a GM and a Player connected and past
// their state.sync, and no scenes yet — most of these tests care about
// what happens as scenes appear.
type sceneTestRoom struct {
	ts     *testServer
	room   store.Room
	gm     *testClient
	player *testClient
	// How many scenes createScene has made, so it knows whether an
	// activation is coming. Asking the store instead would race: the
	// scene.created broadcast goes out before SetActiveScene runs.
	created int
}

func newSceneTestRoom(t *testing.T) *sceneTestRoom {
	t.Helper()

	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	player, err := ts.store.JoinRoom(room.ID, "Bob")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	s := &sceneTestRoom{
		ts:     ts,
		room:   room,
		gm:     ts.connect(t, room.Slug, gm.SessionToken),
		player: ts.connect(t, room.Slug, player.SessionToken),
	}
	s.gm.readEnvelope(t)     // state.sync
	s.player.readEnvelope(t) // state.sync
	return s
}

// createScene drives scene.create over the wire and returns the new
// scene's id, draining the echoes both clients receive.
func (s *sceneTestRoom) createScene(t *testing.T, name string) string {
	t.Helper()

	s.gm.send(t, "scene.create", map[string]any{
		"name": name, "gridSize": 70, "width": 210, "height": 140,
	})

	env := s.gm.readEnvelope(t)
	if env.Type != "scene.created" {
		t.Fatalf("type = %q, want scene.created", env.Type)
	}
	id := sceneIDFromPayload(t, env)
	s.player.readEnvelope(t) // the player's copy of scene.created

	// Only the room's first scene also activates, which is two more
	// envelopes to clear before the next assertion.
	if s.created == 0 {
		s.gm.readEnvelope(t)
		s.player.readEnvelope(t)
	}
	s.created++
	return id
}

func (s *sceneTestRoom) activeSceneID(t *testing.T) string {
	t.Helper()

	room, err := s.ts.store.GetRoomByID(s.room.ID)
	if err != nil {
		t.Fatalf("GetRoomByID: %v", err)
	}
	if room.ActiveSceneID == nil {
		return ""
	}
	return *room.ActiveSceneID
}

func sceneIDFromPayload(t *testing.T, env envelope) string {
	t.Helper()

	var payload struct {
		Scene struct {
			ID string `json:"id"`
		} `json:"scene"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal %s payload: %v", env.Type, err)
	}
	if payload.Scene.ID == "" {
		t.Fatalf("%s payload carried no scene id", env.Type)
	}
	return payload.Scene.ID
}

// The room's very first scene has to take over — there is nothing for a
// GM to switch away from, and a room showing no map at all after they
// just made one would read as a failure.
func TestSceneCreate_FirstSceneActivatesButLaterOnesDoNot(t *testing.T) {
	s := newSceneTestRoom(t)

	s.gm.send(t, "scene.create", map[string]any{
		"name": "Tavern", "gridSize": 70, "width": 210, "height": 140,
	})
	first := sceneIDFromPayload(t, s.gm.readEnvelope(t))
	s.player.readEnvelope(t)

	if env := s.gm.readEnvelope(t); env.Type != "scene.activated" {
		t.Fatalf("type = %q, want the first scene to activate", env.Type)
	}
	s.player.readEnvelope(t)
	if got := s.activeSceneID(t); got != first {
		t.Fatalf("active scene = %q, want the first scene %q", got, first)
	}

	// The second must not drag the party off the map they're on.
	s.gm.send(t, "scene.create", map[string]any{
		"name": "Dungeon", "gridSize": 70, "width": 210, "height": 140,
	})
	if env := s.gm.readEnvelope(t); env.Type != "scene.created" {
		t.Fatalf("type = %q, want scene.created", env.Type)
	}
	s.player.readEnvelope(t)
	// Nothing else should follow: no activation for the second scene.
	s.gm.expectNoMessage(t, 300*time.Millisecond)

	if got := s.activeSceneID(t); got != first {
		t.Fatalf("active scene = %q, want it left on the first scene %q", got, first)
	}
}

// A scene starts fully revealed rather than fully covered — see the
// comment in handleSceneCreate for why: a black rectangle with nothing
// painted on it yet reads as broken, not as "nothing revealed".
func TestSceneCreate_StartsFullyRevealed(t *testing.T) {
	s := newSceneTestRoom(t)
	id := s.createScene(t, "Tavern")

	// 210x140 at gridSize 70 is 3 columns by 2 rows, same bounds fog_test.go
	// uses for sceneFogCells.
	cells, err := s.ts.store.ListFogCells(id)
	if err != nil {
		t.Fatalf("ListFogCells: %v", err)
	}
	if len(cells) != 6 {
		t.Fatalf("len(cells) = %d, want all 6 cells revealed", len(cells))
	}
}

// A scene too large for sceneFogCells's cap (see handleFogRevealAll)
// can't be materialised at creation time either — but that must not
// fail scene creation itself, just leave it starting covered like every
// scene used to.
func TestSceneCreate_TooLargeToMaterialiseStillSucceeds(t *testing.T) {
	s := newSceneTestRoom(t)

	s.gm.send(t, "scene.create", map[string]any{
		"name": "Vast", "gridSize": 1, "width": 1000, "height": 1000,
	})
	env := s.gm.readEnvelope(t)
	if env.Type != "scene.created" {
		t.Fatalf("type = %q, want scene.created", env.Type)
	}
	id := sceneIDFromPayload(t, env)
	s.player.readEnvelope(t)
	s.gm.readEnvelope(t)     // scene.activated: the room's first scene
	s.player.readEnvelope(t)

	cells, err := s.ts.store.ListFogCells(id)
	if err != nil {
		t.Fatalf("ListFogCells: %v", err)
	}
	if len(cells) != 0 {
		t.Fatalf("len(cells) = %d, want none — too large to auto-reveal", len(cells))
	}
}

func TestStateSync_CarriesEverySceneInTheRoom(t *testing.T) {
	s := newSceneTestRoom(t)
	s.createScene(t, "Tavern")
	s.createScene(t, "Dungeon")

	// A fresh connection is what the picker actually renders from.
	joiner, err := s.ts.store.JoinRoom(s.room.ID, "Carol")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	fresh := s.ts.connect(t, s.room.Slug, joiner.SessionToken)

	env := fresh.readEnvelope(t)
	if env.Type != "state.sync" {
		t.Fatalf("type = %q, want state.sync", env.Type)
	}
	var payload struct {
		Scenes []struct {
			Name string `json:"name"`
		} `json:"scenes"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal state.sync payload: %v", err)
	}
	if len(payload.Scenes) != 2 || payload.Scenes[0].Name != "Tavern" {
		t.Fatalf("scenes = %+v, want Tavern then Dungeon", payload.Scenes)
	}
}

func TestSceneDelete_RemovesTheSceneAndTellsTheRoom(t *testing.T) {
	s := newSceneTestRoom(t)
	s.createScene(t, "Tavern")
	doomed := s.createScene(t, "Dungeon")

	s.gm.send(t, "scene.delete", map[string]any{"sceneId": doomed})

	env := s.gm.readEnvelope(t)
	if env.Type != "scene.deleted" {
		t.Fatalf("type = %q, want scene.deleted", env.Type)
	}
	// Players need it too — the picker is GM-only today, but the event is
	// the room's record of what exists.
	if env := s.player.readEnvelope(t); env.Type != "scene.deleted" {
		t.Fatalf("player got %q, want scene.deleted", env.Type)
	}

	scenes, err := s.ts.store.ListScenesForRoom(s.room.ID)
	if err != nil {
		t.Fatalf("ListScenesForRoom: %v", err)
	}
	if len(scenes) != 1 || scenes[0].Name != "Tavern" {
		t.Fatalf("scenes = %+v, want only Tavern left", scenes)
	}
}

// Deleting the scene everyone is looking at would leave the room's
// active_scene_id dangling — there's no foreign key to clean it up — so
// this has to be refused rather than repaired.
func TestSceneDelete_RefusesTheActiveScene(t *testing.T) {
	s := newSceneTestRoom(t)
	active := s.createScene(t, "Tavern")

	s.gm.send(t, "scene.delete", map[string]any{"sceneId": active})

	if msg := errorMessage(t, s.gm.readEnvelope(t)); msg == "" {
		t.Fatal("expected a refusal explaining the scene is active")
	}
	if scenes, err := s.ts.store.ListScenesForRoom(s.room.ID); err != nil || len(scenes) != 1 {
		t.Fatalf("scenes = %+v (err %v), want the scene still there", scenes, err)
	}
}

func TestSceneDelete_PlayerMayNot(t *testing.T) {
	s := newSceneTestRoom(t)
	s.createScene(t, "Tavern")
	target := s.createScene(t, "Dungeon")

	s.player.send(t, "scene.delete", map[string]any{"sceneId": target})

	if msg := errorMessage(t, s.player.readEnvelope(t)); msg == "" {
		t.Fatal("expected a refusal message")
	}
	if scenes, err := s.ts.store.ListScenesForRoom(s.room.ID); err != nil || len(scenes) != 2 {
		t.Fatalf("scenes = %+v (err %v), want both still there", scenes, err)
	}
}

func TestSceneDelete_SceneFromAnotherRoomFailsLikeAMissingOne(t *testing.T) {
	s := newSceneTestRoom(t)
	s.createScene(t, "Tavern")

	otherRoom, _, err := s.ts.store.CreateRoom("Other", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	otherScene, err := s.ts.store.CreateScene(otherRoom.ID, "Theirs", nil, 70, 210, 140)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	s.gm.send(t, "scene.delete", map[string]any{"sceneId": otherScene.ID})
	fromOtherRoom := errorMessage(t, s.gm.readEnvelope(t))

	s.gm.send(t, "scene.delete", map[string]any{"sceneId": "nosuch"})
	fromMissing := errorMessage(t, s.gm.readEnvelope(t))

	if fromOtherRoom != fromMissing {
		t.Fatalf("errors differ: %q vs %q — the two must be indistinguishable", fromOtherRoom, fromMissing)
	}
	if _, err := s.ts.store.GetScene(otherScene.ID); err != nil {
		t.Fatalf("the other room's scene should be untouched: %v", err)
	}
}

func TestSceneSetMap_SwapsTheArtWithoutDisturbingTheScene(t *testing.T) {
	s := newSceneTestRoom(t)
	sceneID := s.createScene(t, "Tavern")

	// A token and some fog stand in for a session's worth of progress,
	// which a map swap must not touch. Cleared first: scene.create now
	// starts a scene fully revealed, and this test wants to assert on
	// exactly one hand-painted cell surviving, not however many the
	// scene's bounds auto-revealed.
	if _, err := s.ts.store.CreateToken(store.Token{
		SceneID: sceneID, Name: "Goblin", X: 2, Y: 3, Width: 1, Height: 1,
	}); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if err := s.ts.store.ClearFog(sceneID); err != nil {
		t.Fatalf("ClearFog: %v", err)
	}
	if err := s.ts.store.RevealCells(sceneID, []store.FogCell{{X: 1, Y: 1}}); err != nil {
		t.Fatalf("RevealCells: %v", err)
	}

	asset, err := s.ts.store.CreateAsset("hash", "new-map.webp", "image/webp", 10)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if err := s.ts.store.AddAssetToRoom(s.room.ID, asset.ID, "", "", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	s.gm.send(t, "scene.setMap", map[string]any{
		"sceneId": sceneID, "mapAssetId": asset.ID, "width": 1400, "height": 1000,
	})

	env := s.gm.readEnvelope(t)
	// Deliberately not scene.activated: that carries the full picture and
	// makes clients discard undo history and in-flight gestures for what
	// is only a change of backdrop.
	if env.Type != "scene.updated" {
		t.Fatalf("type = %q, want scene.updated", env.Type)
	}
	if env := s.player.readEnvelope(t); env.Type != "scene.updated" {
		t.Fatalf("player got %q, want scene.updated", env.Type)
	}

	scene, err := s.ts.store.GetScene(sceneID)
	if err != nil {
		t.Fatalf("GetScene: %v", err)
	}
	if scene.MapAssetID == nil || *scene.MapAssetID != asset.ID {
		t.Fatalf("map asset = %v, want %q", scene.MapAssetID, asset.ID)
	}
	if tokens, err := s.ts.store.ListTokensForScene(sceneID); err != nil || len(tokens) != 1 {
		t.Fatalf("tokens = %+v (err %v), want the token kept", tokens, err)
	}
	if cells, err := s.ts.store.ListFogCells(sceneID); err != nil || len(cells) != 1 {
		t.Fatalf("fog = %+v (err %v), want the revealed cell kept", cells, err)
	}
}

// Same rule as scene and token creation: an asset that exists but
// belongs to another room's library is not usable here, and says so the
// same way a missing one does.
func TestSceneSetMap_RejectsAssetFromAnotherRoom(t *testing.T) {
	s := newSceneTestRoom(t)
	sceneID := s.createScene(t, "Tavern")

	otherRoom, _, err := s.ts.store.CreateRoom("Other", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	asset, err := s.ts.store.CreateAsset("hash", "theirs.webp", "image/webp", 10)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if err := s.ts.store.AddAssetToRoom(otherRoom.ID, asset.ID, "", "", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	s.gm.send(t, "scene.setMap", map[string]any{
		"sceneId": sceneID, "mapAssetId": asset.ID, "width": 100, "height": 100,
	})
	foreign := errorMessage(t, s.gm.readEnvelope(t))

	s.gm.send(t, "scene.setMap", map[string]any{
		"sceneId": sceneID, "mapAssetId": "nosuch", "width": 100, "height": 100,
	})
	missing := errorMessage(t, s.gm.readEnvelope(t))

	if foreign != missing {
		t.Fatalf("errors differ: %q vs %q — the two must be indistinguishable", foreign, missing)
	}

	scene, err := s.ts.store.GetScene(sceneID)
	if err != nil {
		t.Fatalf("GetScene: %v", err)
	}
	if scene.MapAssetID != nil {
		t.Fatalf("map asset = %v, want the scene left alone", scene.MapAssetID)
	}
}

func TestSceneSetMap_PlayerMayNot(t *testing.T) {
	s := newSceneTestRoom(t)
	sceneID := s.createScene(t, "Tavern")

	s.player.send(t, "scene.setMap", map[string]any{
		"sceneId": sceneID, "mapAssetId": nil, "width": 100, "height": 100,
	})

	if msg := errorMessage(t, s.player.readEnvelope(t)); msg == "" {
		t.Fatal("expected a refusal message")
	}
}
