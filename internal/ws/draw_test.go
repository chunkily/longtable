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
	player, err := ts.store.JoinRoom(room.ID, "Bob")
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
	player, err := ts.store.JoinRoom(room.ID, "Bob")
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
	player, err := ts.store.JoinRoom(room.ID, "Bob")
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
	if _, err := ts.store.CreateDrawing(scene.ID, store.DrawingKindLine, []store.Point{{X: 0, Y: 0}, {X: 1, Y: 1}}, "#cc0000", &player.ID); err != nil {
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

func TestDrawCreate_BroadcastsToOtherClients(t *testing.T) {
	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	player, err := ts.store.JoinRoom(room.ID, "Bob")
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
		"kind":    "circle",
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
	if _, err := ts.store.CreateDrawing(scene.ID, store.DrawingKindLine, []store.Point{{X: 0, Y: 0}, {X: 1, Y: 1}}, "#cc0000", nil); err != nil {
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
	player, err := ts.store.JoinRoom(room.ID, "Bob")
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

	d, err := r.ts.store.CreateDrawing(r.scene.ID, store.DrawingKindLine,
		[]store.Point{{X: 0, Y: 0}, {X: 1, Y: 1}}, "#cc0000", author)
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
	player, err := ts.store.JoinRoom(room.ID, "Bob")
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
