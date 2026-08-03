package ws

import (
	"testing"
)

// Asset rows are global so that identical uploads share one stored file,
// which means every room's asset IDs live in a single namespace. What
// keeps one room's art out of another is the room's library — so these
// check the scoping on the two commands that can reference an asset.
//
// The failure this guards against isn't hypothetical: before the library
// existed the check was "does this asset exist", which any room could
// satisfy with any ID it happened to learn.

func TestSceneCreate_RejectsAssetFromAnotherRoom(t *testing.T) {
	ts := newTestServer(t)

	roomA, gmA, err := ts.store.CreateRoom("Room A", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	roomB, gmB, err := ts.store.CreateRoom("Room B", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	asset, err := ts.store.CreateAsset("hash-a", "secret-map.webp", "image/webp", 1024)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if err := ts.store.AddAssetToRoom(roomA.ID, asset.ID, "", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	// Room B knows the ID — say it leaked — but the asset was never added
	// to their library.
	clientB := ts.connect(t, roomB.Slug, gmB.SessionToken)
	clientB.readEnvelope(t) // state.sync
	clientB.send(t, "scene.create", map[string]any{
		"name":       "Stolen",
		"mapAssetId": asset.ID,
		"gridSize":   70,
		"width":      100,
		"height":     100,
	})
	if env := clientB.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}

	// Room A, whose library it is, can use it.
	clientA := ts.connect(t, roomA.Slug, gmA.SessionToken)
	clientA.readEnvelope(t) // state.sync
	clientA.send(t, "scene.create", map[string]any{
		"name":       "Tavern",
		"mapAssetId": asset.ID,
		"gridSize":   70,
		"width":      100,
		"height":     100,
	})
	if env := clientA.readEnvelope(t); env.Type != "scene.created" {
		t.Fatalf("type = %q, want scene.created", env.Type)
	}
}

func TestTokenCreate_RejectsAssetFromAnotherRoom(t *testing.T) {
	ts := newTestServer(t)

	roomA, _, err := ts.store.CreateRoom("Room A", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	roomB, gmB, err := ts.store.CreateRoom("Room B", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	sceneB, err := ts.store.CreateScene(roomB.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	asset, err := ts.store.CreateAsset("hash-a", "goblin.webp", "image/webp", 512)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if err := ts.store.AddAssetToRoom(roomA.ID, asset.ID, "", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	client := ts.connect(t, roomB.Slug, gmB.SessionToken)
	client.readEnvelope(t) // state.sync
	client.send(t, "token.create", map[string]any{
		"sceneId":      sceneB.ID,
		"name":         "Goblin",
		"imageAssetId": asset.ID,
		"x":            1,
		"y":            1,
	})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}

	// Once it's in their library, the same command works — so this is
	// scoping, not a blanket refusal.
	if err := ts.store.AddAssetToRoom(roomB.ID, asset.ID, "", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}
	client.send(t, "token.create", map[string]any{
		"sceneId":      sceneB.ID,
		"name":         "Goblin",
		"imageAssetId": asset.ID,
		"x":            1,
		"y":            1,
	})
	if env := client.readEnvelope(t); env.Type != "token.created" {
		t.Fatalf("type = %q, want token.created", env.Type)
	}
}

// An ID that doesn't exist at all and one that belongs to someone else
// have to be indistinguishable, or the error becomes a way to ask what
// exists in other rooms.
func TestAssetScope_UnknownAndForeignAssetsFailAlike(t *testing.T) {
	ts := newTestServer(t)

	roomA, _, err := ts.store.CreateRoom("Room A", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	roomB, gmB, err := ts.store.CreateRoom("Room B", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	asset, err := ts.store.CreateAsset("hash-a", "map.webp", "image/webp", 1)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if err := ts.store.AddAssetToRoom(roomA.ID, asset.ID, "", "", nil); err != nil {
		t.Fatalf("AddAssetToRoom: %v", err)
	}

	client := ts.connect(t, roomB.Slug, gmB.SessionToken)
	client.readEnvelope(t) // state.sync

	var messages []string
	for _, assetID := range []string{asset.ID, "no-such-asset-id"} {
		client.send(t, "scene.create", map[string]any{
			"name":       "Scene",
			"mapAssetId": assetID,
			"gridSize":   70,
		})
		env := client.readEnvelope(t)
		if env.Type != "error" {
			t.Fatalf("type = %q, want error", env.Type)
		}
		messages = append(messages, string(env.Payload))
	}

	if messages[0] != messages[1] {
		t.Fatalf("a foreign asset and a missing one gave different errors:\n %s\n %s",
			messages[0], messages[1])
	}
}

// Nothing above should suggest an asset is required: a scene or token
// with no image is ordinary and must stay allowed.
func TestSceneCreate_NoAssetIsStillFine(t *testing.T) {
	ts := newTestServer(t)

	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	client := ts.connect(t, room.Slug, gm.SessionToken)
	client.readEnvelope(t) // state.sync
	client.send(t, "scene.create", map[string]any{"name": "Blank", "gridSize": 70})

	if env := client.readEnvelope(t); env.Type != "scene.created" {
		t.Fatalf("type = %q, want scene.created", env.Type)
	}
}
