package ws

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/coder/websocket"

	"longtable/internal/store"
)

// measureTestRoom is a room with a scene, a GM and a Player, both
// connected and past their state.sync — the setup every measuring test
// needs before it can say anything about the relay.
type measureTestRoom struct {
	ts     *testServer
	scene  store.Scene
	player store.Participant

	gmClient     *testClient
	playerClient *testClient
}

func newMeasureTestRoom(t *testing.T) *measureTestRoom {
	t.Helper()

	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "", "password")
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

	return &measureTestRoom{
		ts:           ts,
		scene:        scene,
		player:       player,
		gmClient:     gmClient,
		playerClient: playerClient,
	}
}

type measureUpdatedPayload struct {
	ParticipantID   string `json:"participantId"`
	ParticipantName string `json:"participantName"`
	SceneID         string `json:"sceneId"`
	From            struct {
		X, Y float64
	} `json:"from"`
	To struct {
		X, Y float64
	} `json:"to"`
	Kind      string  `json:"kind"`
	WidthFeet float64 `json:"widthFeet"`
}

// Measuring is open to everyone, not just the GM: knowing how far away
// something is is what a player needs it for.
func TestMeasureUpdate_RelaysEndpointsWithoutPersisting(t *testing.T) {
	r := newMeasureTestRoom(t)

	r.playerClient.send(t, "measure.update", map[string]any{
		"sceneId": r.scene.ID,
		"from":    map[string]float64{"x": 10, "y": 20},
		"to":      map[string]float64{"x": 300, "y": 160},
	})

	r.playerClient.readEnvelope(t) // echoed back to the sender
	env := r.gmClient.readEnvelope(t)
	if env.Type != "measure.updated" {
		t.Fatalf("type = %q, want measure.updated", env.Type)
	}

	var payload measureUpdatedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal measure.updated payload: %v", err)
	}
	if payload.ParticipantID != r.player.ID {
		t.Fatalf("participantId = %q, want %q", payload.ParticipantID, r.player.ID)
	}
	if payload.ParticipantName != "Bob" {
		t.Fatalf("participantName = %q, want Bob", payload.ParticipantName)
	}
	if payload.SceneID != r.scene.ID {
		t.Fatalf("sceneId = %q, want %q", payload.SceneID, r.scene.ID)
	}
	if payload.From.X != 10 || payload.From.Y != 20 || payload.To.X != 300 || payload.To.Y != 160 {
		t.Fatalf("endpoints = (%v,%v)-(%v,%v), want (10,20)-(300,160)",
			payload.From.X, payload.From.Y, payload.To.X, payload.To.Y)
	}

	// A measurement is meaningless once the drag is over, so it must
	// leave nothing behind on the scene.
	drawings, err := r.ts.store.ListDrawingsForScene(r.scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	if len(drawings) != 0 {
		t.Fatalf("len(drawings) = %d, want 0 (measurements must not be persisted)", len(drawings))
	}

	// An update that names no kind is the plain distance line the tool
	// started as, so an older client keeps working untouched.
	if payload.Kind != "distance" {
		t.Fatalf("kind = %q, want the distance default", payload.Kind)
	}
}

// Area templates ride the same relay as the distance line: same
// lifecycle, same one-per-participant keying, same cleanup. Only the
// shape differs, so the kind has to survive the round trip.
func TestMeasureUpdate_CarriesTheTemplateShapeAndWidth(t *testing.T) {
	r := newMeasureTestRoom(t)

	r.playerClient.send(t, "measure.update", map[string]any{
		"sceneId":   r.scene.ID,
		"kind":      "line",
		"from":      map[string]float64{"x": 0, "y": 0},
		"to":        map[string]float64{"x": 420, "y": 0},
		"widthFeet": 10,
	})

	r.playerClient.readEnvelope(t)
	env := r.gmClient.readEnvelope(t)

	var payload measureUpdatedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal measure.updated payload: %v", err)
	}
	if payload.Kind != "line" {
		t.Fatalf("kind = %q, want line", payload.Kind)
	}
	// Feet rather than world units, so the width means the same thing on
	// a scene with a different grid size.
	if payload.WidthFeet != 10 {
		t.Fatalf("widthFeet = %v, want 10", payload.WidthFeet)
	}
}

func TestMeasureUpdate_RejectsAnUnknownShape(t *testing.T) {
	r := newMeasureTestRoom(t)

	r.playerClient.send(t, "measure.update", map[string]any{
		"sceneId": r.scene.ID,
		"kind":    "pyramid",
		"from":    map[string]float64{"x": 0, "y": 0},
		"to":      map[string]float64{"x": 70, "y": 0},
	})

	env := r.playerClient.readEnvelope(t)
	if env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
	// Nothing should have reached the rest of the room.
	r.gmClient.expectNoMessage(t, 200*time.Millisecond)
}

func TestMeasureEnd_TellsEveryoneWhoseMeasurementEnded(t *testing.T) {
	r := newMeasureTestRoom(t)

	r.playerClient.send(t, "measure.end", map[string]any{})

	r.playerClient.readEnvelope(t) // echoed back to the sender
	env := r.gmClient.readEnvelope(t)
	if env.Type != "measure.ended" {
		t.Fatalf("type = %q, want measure.ended", env.Type)
	}

	var payload struct {
		ParticipantID string `json:"participantId"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal measure.ended payload: %v", err)
	}
	// Taken from the connection, not the payload: nobody gets to clear
	// someone else's measurement.
	if payload.ParticipantID != r.player.ID {
		t.Fatalf("participantId = %q, want %q", payload.ParticipantID, r.player.ID)
	}
}

// A client that drops mid-drag never sends measure.end, so without this
// its line would hang on every other map until the scene changed.
func TestMeasure_DisconnectEndsTheMeasurement(t *testing.T) {
	r := newMeasureTestRoom(t)

	r.playerClient.send(t, "measure.update", map[string]any{
		"sceneId": r.scene.ID,
		"from":    map[string]float64{"x": 0, "y": 0},
		"to":      map[string]float64{"x": 70, "y": 70},
	})
	if env := r.gmClient.readEnvelope(t); env.Type != "measure.updated" {
		t.Fatalf("type = %q, want measure.updated", env.Type)
	}

	if err := r.playerClient.conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatalf("close: %v", err)
	}

	env := r.gmClient.readEnvelope(t)
	if env.Type != "measure.ended" {
		t.Fatalf("type = %q, want measure.ended", env.Type)
	}
	var payload struct {
		ParticipantID string `json:"participantId"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal measure.ended payload: %v", err)
	}
	if payload.ParticipantID != r.player.ID {
		t.Fatalf("participantId = %q, want %q", payload.ParticipantID, r.player.ID)
	}
}

func TestMeasureUpdate_RejectsSceneFromAnotherRoom(t *testing.T) {
	ts := newTestServer(t)
	roomA, _, err := ts.store.CreateRoom("Room A", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	roomB, gmB, err := ts.store.CreateRoom("Room B", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	sceneA, err := ts.store.CreateScene(roomA.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	clientB := ts.connect(t, roomB.Slug, gmB.SessionToken)
	clientB.readEnvelope(t) // state.sync

	clientB.send(t, "measure.update", map[string]any{
		"sceneId": sceneA.ID,
		"from":    map[string]float64{"x": 0, "y": 0},
		"to":      map[string]float64{"x": 1, "y": 1},
	})

	if env := clientB.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
}

func TestMeasureUpdate_RejectsMissingScene(t *testing.T) {
	r := newMeasureTestRoom(t)

	r.playerClient.send(t, "measure.update", map[string]any{
		"from": map[string]float64{"x": 0, "y": 0},
		"to":   map[string]float64{"x": 1, "y": 1},
	})

	if env := r.playerClient.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
}
