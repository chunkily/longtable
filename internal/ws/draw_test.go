package ws

import (
	"encoding/json"
	"testing"

	"longtable/internal/store"
)

func TestDrawCreate_PlayerCanDrawAndItPersists(t *testing.T) {
	ts := newTestServer(t)
	room, _, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	player, err := ts.store.JoinRoom(room.ID, "Bob", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	scene, err := ts.store.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	client := ts.connect(t, room.Slug, player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "draw.create", map[string]any{
		"sceneId": scene.ID,
		"kind":    "freehand",
		"points":  []map[string]float64{{"x": 1, "y": 2}, {"x": 3, "y": 4}, {"x": 5, "y": 6}},
	})

	env := client.readEnvelope(t)
	if env.Type != "drawing.created" {
		t.Fatalf("type = %q, want drawing.created", env.Type)
	}
	var payload struct {
		Kind   string `json:"kind"`
		Color  string `json:"color"`
		Points []struct {
			X, Y float64
		} `json:"points"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal drawing.created payload: %v", err)
	}
	if payload.Kind != "freehand" {
		t.Fatalf("kind = %q, want freehand", payload.Kind)
	}
	if payload.Color == "" {
		t.Fatal("expected a default color to be applied")
	}
	if len(payload.Points) != 3 {
		t.Fatalf("len(points) = %d, want 3", len(payload.Points))
	}

	drawings, err := ts.store.ListDrawingsForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	if len(drawings) != 1 {
		t.Fatalf("len(drawings) = %d, want 1 (should be persisted)", len(drawings))
	}
}

func TestDrawCreate_RecordsAuthorFromSession(t *testing.T) {
	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	player, err := ts.store.JoinRoom(room.ID, "Bob", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	scene, err := ts.store.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	client := ts.connect(t, room.Slug, player.SessionToken)
	client.readEnvelope(t) // state.sync

	// The author comes from the authenticated session, so claiming
	// someone else's ID in the payload must not stick.
	gmParticipant, err := ts.store.GetParticipantByToken(room.ID, gm.SessionToken)
	if err != nil {
		t.Fatalf("GetParticipantByToken: %v", err)
	}
	client.send(t, "draw.create", map[string]any{
		"sceneId":                scene.ID,
		"kind":                   "line",
		"points":                 []map[string]float64{{"x": 0, "y": 0}, {"x": 1, "y": 1}},
		"createdByParticipantId": gmParticipant.ID,
	})

	env := client.readEnvelope(t)
	if env.Type != "drawing.created" {
		t.Fatalf("type = %q, want drawing.created", env.Type)
	}
	var payload struct {
		CreatedByParticipantID *string `json:"createdByParticipantId"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal drawing.created payload: %v", err)
	}
	if payload.CreatedByParticipantID == nil || *payload.CreatedByParticipantID != player.ID {
		t.Fatalf("broadcast createdByParticipantId = %v, want %q", payload.CreatedByParticipantID, player.ID)
	}

	drawings, err := ts.store.ListDrawingsForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	if len(drawings) != 1 {
		t.Fatalf("len(drawings) = %d, want 1", len(drawings))
	}
	if got := drawings[0].CreatedByParticipantID; got == nil || *got != player.ID {
		t.Fatalf("persisted CreatedByParticipantID = %v, want %q", got, player.ID)
	}
}

func TestStateSync_DrawingsIncludeAuthor(t *testing.T) {
	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	player, err := ts.store.JoinRoom(room.ID, "Bob", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	scene, err := ts.store.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	if err := ts.store.SetActiveScene(room.ID, scene.ID); err != nil {
		t.Fatalf("SetActiveScene: %v", err)
	}
	if _, err := ts.store.CreateDrawing(store.Drawing{SceneID: scene.ID, Kind: store.DrawingKindLine, Points: []store.Point{{X: 0, Y: 0}, {X: 1, Y: 1}}, Color: "#cc0000", CreatedByParticipantID: &player.ID}); err != nil {
		t.Fatalf("CreateDrawing: %v", err)
	}

	client := ts.connect(t, room.Slug, gm.SessionToken)
	env := client.readEnvelope(t)

	var payload struct {
		Drawings []struct {
			CreatedByParticipantID *string `json:"createdByParticipantId"`
		} `json:"drawings"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal state.sync payload: %v", err)
	}
	if len(payload.Drawings) != 1 {
		t.Fatalf("len(drawings) = %d, want 1", len(payload.Drawings))
	}
	if got := payload.Drawings[0].CreatedByParticipantID; got == nil || *got != player.ID {
		t.Fatalf("createdByParticipantId = %v, want %q", got, player.ID)
	}
}

// A client that has already drawn the stroke locally sends the id it
// used, and gets that exact id back — otherwise it can't tell its own
// echo from someone else's new drawing.
func TestDrawCreate_UsesClientSuppliedID(t *testing.T) {
	r := newDrawTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	const id = "6f1e3b8a-2c4d-4f1e-9a7b-0d5c8e2f4a13"
	client.send(t, "draw.create", map[string]any{
		"drawingId": id,
		"sceneId":   r.scene.ID,
		"kind":      "line",
		"points":    []map[string]float64{{"x": 0, "y": 0}, {"x": 1, "y": 1}},
	})

	env := client.readEnvelope(t)
	if env.Type != "drawing.created" {
		t.Fatalf("type = %q, want drawing.created", env.Type)
	}
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal drawing.created payload: %v", err)
	}
	if payload.ID != id {
		t.Fatalf("id = %q, want the client's %q", payload.ID, id)
	}

	stored, err := r.ts.store.GetDrawing(id)
	if err != nil {
		t.Fatalf("GetDrawing(client id): %v", err)
	}
	if stored.SceneID != r.scene.ID {
		t.Fatalf("stored under the wrong scene: %+v", stored)
	}
}

func TestDrawCreate_RejectsMalformedAndDuplicateIDs(t *testing.T) {
	r := newDrawTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	// Only the canonical spelling is accepted, so the id echoed in any
	// failure is byte-identical to the one the client is holding.
	for _, id := range []string{
		"not-a-uuid",
		"{6f1e3b8a-2c4d-4f1e-9a7b-0d5c8e2f4a13}",
		"6F1E3B8A-2C4D-4F1E-9A7B-0D5C8E2F4A13",
	} {
		client.send(t, "draw.create", map[string]any{
			"drawingId": id,
			"sceneId":   r.scene.ID,
			"kind":      "line",
			"points":    []map[string]float64{{"x": 0, "y": 0}, {"x": 1, "y": 1}},
		})
		env := client.readEnvelope(t)
		if env.Type != "error" {
			t.Fatalf("drawingId %q: type = %q, want error", id, env.Type)
		}
		if got := errorDrawingID(t, env); got != id {
			t.Fatalf("error drawingId = %q, want %q", got, id)
		}
	}

	const valid = "6f1e3b8a-2c4d-4f1e-9a7b-0d5c8e2f4a13"
	create := map[string]any{
		"drawingId": valid,
		"sceneId":   r.scene.ID,
		"kind":      "line",
		"points":    []map[string]float64{{"x": 0, "y": 0}, {"x": 1, "y": 1}},
	}
	client.send(t, "draw.create", create)
	if env := client.readEnvelope(t); env.Type != "drawing.created" {
		t.Fatalf("type = %q, want drawing.created", env.Type)
	}

	// Reusing an id is the primary key's problem to catch.
	client.send(t, "draw.create", create)
	env := client.readEnvelope(t)
	if env.Type != "error" {
		t.Fatalf("duplicate id: type = %q, want error", env.Type)
	}
	if got := errorDrawingID(t, env); got != valid {
		t.Fatalf("error drawingId = %q, want %q", got, valid)
	}
	if n := r.drawingCount(t); n != 1 {
		t.Fatalf("len(drawings) = %d, want 1", n)
	}
}

// Every rejection a client could have rendered optimistically names the
// drawing, so exactly that stroke can be taken back off the map.
func TestDrawCreate_RejectionsNameTheDrawing(t *testing.T) {
	r := newDrawTestRoom(t)
	otherRoom, _, err := r.ts.store.CreateRoom("Room B", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	otherScene, err := r.ts.store.CreateScene(otherRoom.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	const id = "1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed"
	line := []map[string]float64{{"x": 0, "y": 0}, {"x": 1, "y": 1}}
	rejections := map[string]map[string]any{
		"unknown kind":    {"drawingId": id, "sceneId": r.scene.ID, "kind": "trapezoid", "points": line},
		"wrong points":    {"drawingId": id, "sceneId": r.scene.ID, "kind": "rect", "points": []map[string]float64{{"x": 0, "y": 0}}},
		"scene elsewhere": {"drawingId": id, "sceneId": otherScene.ID, "kind": "line", "points": line},
	}
	for name, payload := range rejections {
		client.send(t, "draw.create", payload)
		env := client.readEnvelope(t)
		if env.Type != "error" {
			t.Fatalf("%s: type = %q, want error", name, env.Type)
		}
		if got := errorDrawingID(t, env); got != id {
			t.Fatalf("%s: error drawingId = %q, want %q", name, got, id)
		}
	}
	if n := r.drawingCount(t); n != 0 {
		t.Fatalf("len(drawings) = %d, want 0", n)
	}
}

func TestDrawDelete_RejectionsNameTheDrawing(t *testing.T) {
	r := newDrawTestRoom(t)
	gmDrawing := r.drawing(t, nil)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	// A player erasing what isn't theirs, and anyone erasing what isn't
	// there: both have to come back named, so an optimistic client can
	// put the stroke back.
	for name, id := range map[string]string{
		"not yours":   gmDrawing.ID,
		"not a thing": "0f9c1a2b-3d4e-4f50-8a6b-7c8d9e0f1a2b",
	} {
		client.send(t, "draw.delete", map[string]any{"drawingId": id})
		env := client.readEnvelope(t)
		if env.Type != "error" {
			t.Fatalf("%s: type = %q, want error", name, env.Type)
		}
		if got := errorDrawingID(t, env); got != id {
			t.Fatalf("%s: error drawingId = %q, want %q", name, got, id)
		}
	}
	if n := r.drawingCount(t); n != 1 {
		t.Fatalf("len(drawings) = %d, want 1", n)
	}
}

func errorDrawingID(t *testing.T, env envelope) string {
	t.Helper()

	var payload struct {
		Message   string `json:"message"`
		DrawingID string `json:"drawingId"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal error payload: %v", err)
	}
	if payload.Message == "" {
		t.Fatal("error payload has no message")
	}
	return payload.DrawingID
}

func TestDrawCreate_BroadcastsToOtherClients(t *testing.T) {
	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	player, err := ts.store.JoinRoom(room.ID, "Bob", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	scene, err := ts.store.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	gmClient := ts.connect(t, room.Slug, gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := ts.connect(t, room.Slug, player.SessionToken)
	playerClient.readEnvelope(t) // state.sync

	gmClient.send(t, "draw.create", map[string]any{
		"sceneId": scene.ID,
		"kind":    "line",
		"points":  []map[string]float64{{"x": 0, "y": 0}, {"x": 10, "y": 10}},
	})

	gmClient.readEnvelope(t) // drawing.created echoed back to sender
	env := playerClient.readEnvelope(t)
	if env.Type != "drawing.created" {
		t.Fatalf("player did not receive drawing.created, got type = %q", env.Type)
	}
}

func TestDrawCreate_RejectsUnknownKind(t *testing.T) {
	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := ts.store.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	client := ts.connect(t, room.Slug, gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "draw.create", map[string]any{
		"sceneId": scene.ID,
		"kind":    "triangle",
		"points":  []map[string]float64{{"x": 0, "y": 0}, {"x": 1, "y": 1}},
	})
	env := client.readEnvelope(t)
	if env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
}

func TestDrawCreate_RejectsWrongPointCountForFixedShapes(t *testing.T) {
	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := ts.store.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	client := ts.connect(t, room.Slug, gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "draw.create", map[string]any{
		"sceneId": scene.ID,
		"kind":    "rect",
		"points":  []map[string]float64{{"x": 0, "y": 0}, {"x": 1, "y": 1}, {"x": 2, "y": 2}},
	})
	env := client.readEnvelope(t)
	if env.Type != "error" {
		t.Fatalf("type = %q, want error (rect needs exactly 2 points)", env.Type)
	}

	drawings, err := ts.store.ListDrawingsForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	if len(drawings) != 0 {
		t.Fatalf("len(drawings) = %d, want 0", len(drawings))
	}
}

func TestDrawCreate_RejectsSceneFromAnotherRoom(t *testing.T) {
	ts := newTestServer(t)
	roomA, _, err := ts.store.CreateRoom("Room A", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	roomB, gmB, err := ts.store.CreateRoom("Room B", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	sceneA, err := ts.store.CreateScene(roomA.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	clientB := ts.connect(t, roomB.Slug, gmB.SessionToken)
	clientB.readEnvelope(t) // state.sync

	clientB.send(t, "draw.create", map[string]any{
		"sceneId": sceneA.ID,
		"kind":    "line",
		"points":  []map[string]float64{{"x": 0, "y": 0}, {"x": 1, "y": 1}},
	})
	env := clientB.readEnvelope(t)
	if env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}

	drawings, err := ts.store.ListDrawingsForScene(sceneA.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	if len(drawings) != 0 {
		t.Fatalf("len(drawings) = %d, want 0", len(drawings))
	}
}

func TestDrawCreate_UsesGivenColorWhenProvided(t *testing.T) {
	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := ts.store.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	client := ts.connect(t, room.Slug, gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "draw.create", map[string]any{
		"sceneId": scene.ID,
		"kind":    "ellipse",
		"points":  []map[string]float64{{"x": 0, "y": 0}, {"x": 5, "y": 0}},
		"color":   "#00ff00",
	})
	env := client.readEnvelope(t)
	var payload struct {
		Color string `json:"color"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Color != "#00ff00" {
		t.Fatalf("color = %q, want #00ff00", payload.Color)
	}
}

func TestStateSync_IncludesDrawings(t *testing.T) {
	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := ts.store.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	if err := ts.store.SetActiveScene(room.ID, scene.ID); err != nil {
		t.Fatalf("SetActiveScene: %v", err)
	}
	if _, err := ts.store.CreateDrawing(store.Drawing{SceneID: scene.ID, Kind: store.DrawingKindLine, Points: []store.Point{{X: 0, Y: 0}, {X: 1, Y: 1}}, Color: "#cc0000"}); err != nil {
		t.Fatalf("CreateDrawing: %v", err)
	}

	client := ts.connect(t, room.Slug, gm.SessionToken)
	env := client.readEnvelope(t)
	if env.Type != "state.sync" {
		t.Fatalf("type = %q, want state.sync", env.Type)
	}

	var payload struct {
		Drawings []json.RawMessage `json:"drawings"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal state.sync payload: %v", err)
	}
	if len(payload.Drawings) != 1 {
		t.Fatalf("len(drawings) = %d, want 1", len(payload.Drawings))
	}
}

// drawTestRoom sets up a room with a GM, a Player, and a scene — the
// shared fixture for the erase-permission tests below.
type drawTestRoom struct {
	ts     *testServer
	room   store.Room
	gm     store.Participant
	player store.Participant
	scene  store.Scene
}

func newDrawTestRoom(t *testing.T) drawTestRoom {
	t.Helper()

	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	player, err := ts.store.JoinRoom(room.ID, "Bob", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	scene, err := ts.store.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	return drawTestRoom{ts: ts, room: room, gm: gm, player: player, scene: scene}
}

func (r drawTestRoom) drawing(t *testing.T, author *string) store.Drawing {
	t.Helper()

	d, err := r.ts.store.CreateDrawing(store.Drawing{SceneID: r.scene.ID, Kind: store.DrawingKindLine, Points: []store.Point{{X: 0, Y: 0}, {X: 1, Y: 1}}, Color: "#cc0000", CreatedByParticipantID: author})
	if err != nil {
		t.Fatalf("CreateDrawing: %v", err)
	}
	return d
}

func (r drawTestRoom) drawingCount(t *testing.T) int {
	t.Helper()

	drawings, err := r.ts.store.ListDrawingsForScene(r.scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	return len(drawings)
}

func TestDrawDelete_GMErasesAnyonesDrawing(t *testing.T) {
	r := newDrawTestRoom(t)
	drawing := r.drawing(t, &r.player.ID)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync

	gmClient.send(t, "draw.delete", map[string]any{"drawingId": drawing.ID})

	env := gmClient.readEnvelope(t)
	if env.Type != "drawing.deleted" {
		t.Fatalf("type = %q, want drawing.deleted", env.Type)
	}
	var payload struct {
		DrawingID string `json:"drawingId"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal drawing.deleted payload: %v", err)
	}
	if payload.DrawingID != drawing.ID {
		t.Fatalf("drawingId = %q, want %q", payload.DrawingID, drawing.ID)
	}

	// Everyone in the room sees it go, and it stays gone server-side.
	if env := playerClient.readEnvelope(t); env.Type != "drawing.deleted" {
		t.Fatalf("player did not receive drawing.deleted, got type = %q", env.Type)
	}
	if n := r.drawingCount(t); n != 0 {
		t.Fatalf("len(drawings) = %d, want 0", n)
	}
}

func TestDrawDelete_PlayerErasesOwnDrawing(t *testing.T) {
	r := newDrawTestRoom(t)
	drawing := r.drawing(t, &r.player.ID)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "draw.delete", map[string]any{"drawingId": drawing.ID})

	if env := client.readEnvelope(t); env.Type != "drawing.deleted" {
		t.Fatalf("type = %q, want drawing.deleted", env.Type)
	}
	if n := r.drawingCount(t); n != 0 {
		t.Fatalf("len(drawings) = %d, want 0", n)
	}
}

func TestDrawDelete_PlayerCannotEraseSomeoneElsesDrawing(t *testing.T) {
	r := newDrawTestRoom(t)
	gmParticipant, err := r.ts.store.GetParticipantByToken(r.room.ID, r.gm.SessionToken)
	if err != nil {
		t.Fatalf("GetParticipantByToken: %v", err)
	}
	drawing := r.drawing(t, &gmParticipant.ID)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "draw.delete", map[string]any{"drawingId": drawing.ID})

	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
	if n := r.drawingCount(t); n != 1 {
		t.Fatalf("len(drawings) = %d, want 1 (the GM's drawing must survive)", n)
	}
}

// A drawing with no recorded author belongs to nobody, so a Player
// can't claim it — only a GM can clear it.
func TestDrawDelete_PlayerCannotEraseUnattributedDrawing(t *testing.T) {
	r := newDrawTestRoom(t)
	drawing := r.drawing(t, nil)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "draw.delete", map[string]any{"drawingId": drawing.ID})

	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
	if n := r.drawingCount(t); n != 1 {
		t.Fatalf("len(drawings) = %d, want 1", n)
	}

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	gmClient.send(t, "draw.delete", map[string]any{"drawingId": drawing.ID})

	if env := gmClient.readEnvelope(t); env.Type != "drawing.deleted" {
		t.Fatalf("GM erase of an unattributed drawing: type = %q, want drawing.deleted", env.Type)
	}
	if n := r.drawingCount(t); n != 0 {
		t.Fatalf("len(drawings) = %d, want 0", n)
	}
}

func TestDrawDelete_RejectsDrawingFromAnotherRoom(t *testing.T) {
	r := newDrawTestRoom(t)
	drawing := r.drawing(t, nil)

	// A GM of a different room can't reach this one's drawings by ID.
	otherRoom, otherGM, err := r.ts.store.CreateRoom("Room B", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	client := r.ts.connect(t, otherRoom.Slug, otherGM.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "draw.delete", map[string]any{"drawingId": drawing.ID})

	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
	if n := r.drawingCount(t); n != 1 {
		t.Fatalf("len(drawings) = %d, want 1", n)
	}
}

func TestDrawDelete_RejectsUnknownDrawing(t *testing.T) {
	r := newDrawTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "draw.delete", map[string]any{"drawingId": "nope"})

	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
}

func TestPing_BroadcastsWithoutPersisting(t *testing.T) {
	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	player, err := ts.store.JoinRoom(room.ID, "Bob", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	scene, err := ts.store.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	gmClient := ts.connect(t, room.Slug, gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := ts.connect(t, room.Slug, player.SessionToken)
	playerClient.readEnvelope(t) // state.sync

	// Non-GM participants must be able to ping too.
	playerClient.send(t, "ping", map[string]any{"sceneId": scene.ID, "x": 42.0, "y": 99.0})

	playerClient.readEnvelope(t) // echoed back to sender
	env := gmClient.readEnvelope(t)
	if env.Type != "ping" {
		t.Fatalf("type = %q, want ping", env.Type)
	}

	var payload struct {
		X               float64 `json:"x"`
		Y               float64 `json:"y"`
		ParticipantName string  `json:"participantName"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal ping payload: %v", err)
	}
	if payload.X != 42 || payload.Y != 99 {
		t.Fatalf("ping coords = (%v, %v), want (42, 99)", payload.X, payload.Y)
	}
	if payload.ParticipantName != "Bob" {
		t.Fatalf("participantName = %q, want Bob", payload.ParticipantName)
	}

	drawings, err := ts.store.ListDrawingsForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	if len(drawings) != 0 {
		t.Fatalf("len(drawings) = %d, want 0 (pings must not be persisted)", len(drawings))
	}
}

func TestPing_RejectsSceneFromAnotherRoom(t *testing.T) {
	ts := newTestServer(t)
	roomA, _, err := ts.store.CreateRoom("Room A", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	roomB, gmB, err := ts.store.CreateRoom("Room B", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	sceneA, err := ts.store.CreateScene(roomA.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	clientB := ts.connect(t, roomB.Slug, gmB.SessionToken)
	clientB.readEnvelope(t) // state.sync

	clientB.send(t, "ping", map[string]any{"sceneId": sceneA.ID, "x": 1.0, "y": 1.0})
	env := clientB.readEnvelope(t)
	if env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
}

// drawRoom is a room, a scene and one connected player, for the cases
// below that only care what comes back from draw.create.
func drawRoom(t *testing.T) (*testServer, *testClient, store.Scene) {
	t.Helper()
	ts := newTestServer(t)
	room, _, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	player, err := ts.store.JoinRoom(room.ID, "Bob", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	scene, err := ts.store.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	client := ts.connect(t, room.Slug, player.SessionToken)
	client.readEnvelope(t) // state.sync
	return ts, client, scene
}

// createdDrawing sends one draw.create and returns the fill and width the
// room is told about.
func createdDrawing(t *testing.T, client *testClient, payload map[string]any) (bool, float64) {
	t.Helper()
	client.send(t, "draw.create", payload)

	env := client.readEnvelope(t)
	if env.Type != "drawing.created" {
		t.Fatalf("type = %q, want drawing.created", env.Type)
	}
	var got struct {
		Filled      bool    `json:"filled"`
		StrokeWidth float64 `json:"strokeWidth"`
	}
	if err := json.Unmarshal(env.Payload, &got); err != nil {
		t.Fatalf("unmarshal drawing.created payload: %v", err)
	}
	return got.Filled, got.StrokeWidth
}

func twoPoints() []map[string]float64 {
	return []map[string]float64{{"x": 1, "y": 2}, {"x": 3, "y": 4}}
}

func TestDrawCreate_KeepsAFillOnTheKindsThatEncloseAnArea(t *testing.T) {
	for _, kind := range []string{"rect", "ellipse"} {
		t.Run(kind, func(t *testing.T) {
			_, client, scene := drawRoom(t)
			filled, _ := createdDrawing(t, client, map[string]any{
				"sceneId": scene.ID,
				"kind":    kind,
				"points":  twoPoints(),
				"filled":  true,
			})
			if !filled {
				t.Errorf("a filled %s came back unfilled", kind)
			}
		})
	}
}

// A fill on a line or a freehand stroke describes nothing — Konva would
// close the path and shade whatever the stroke happened to loop around —
// so it is dropped rather than refused: the client asked for a drawing,
// and the drawing it meant is the useful answer.
func TestDrawCreate_DropsAFillOnTheKindsThatEncloseNothing(t *testing.T) {
	for _, kind := range []string{"line", "freehand"} {
		t.Run(kind, func(t *testing.T) {
			_, client, scene := drawRoom(t)
			filled, _ := createdDrawing(t, client, map[string]any{
				"sceneId": scene.ID,
				"kind":    kind,
				"points":  twoPoints(),
				"filled":  true,
			})
			if filled {
				t.Errorf("a filled %s should have been normalised to unfilled", kind)
			}
		})
	}
}

func TestDrawCreate_StrokeWidthRoundTripsAndPersists(t *testing.T) {
	ts, client, scene := drawRoom(t)

	_, width := createdDrawing(t, client, map[string]any{
		"sceneId":     scene.ID,
		"kind":        "line",
		"points":      twoPoints(),
		"strokeWidth": 8,
	})
	if width != 8 {
		t.Errorf("strokeWidth = %v, want 8", width)
	}

	drawings, err := ts.store.ListDrawingsForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	if len(drawings) != 1 || drawings[0].StrokeWidth != 8 {
		t.Fatalf("stored stroke width = %v, want 8", drawings)
	}
}

// A width is world pixels on a map everyone shares, so an absurd one
// isn't only the sender's problem — and a Player can't erase the GM's
// drawings, or anyone else's.
func TestDrawCreate_ClampsAnAbsurdStrokeWidth(t *testing.T) {
	cases := []struct {
		name string
		sent any
		want float64
	}{
		{"omitted takes the default", nil, defaultDrawingStrokeWidth},
		{"zero takes the default", 0, defaultDrawingStrokeWidth},
		{"negative takes the default", -5, defaultDrawingStrokeWidth},
		{"below the floor comes up", 0.1, minDrawingStrokeWidth},
		{"above the ceiling comes down", 10000, maxDrawingStrokeWidth},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, client, scene := drawRoom(t)
			payload := map[string]any{
				"sceneId": scene.ID,
				"kind":    "line",
				"points":  twoPoints(),
			}
			if tc.sent != nil {
				payload["strokeWidth"] = tc.sent
			}
			_, width := createdDrawing(t, client, payload)
			if width != tc.want {
				t.Errorf("strokeWidth = %v, want %v", width, tc.want)
			}
		})
	}
}
