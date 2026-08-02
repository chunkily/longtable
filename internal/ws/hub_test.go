package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"longtable/internal/db"
	"longtable/internal/store"
)

// testServer wires a real Store (temp SQLite file) to a Hub and serves
// it over httptest, so these tests exercise the actual WS protocol
// end-to-end rather than calling handlers directly.
type testServer struct {
	store *store.Store
	url   string
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()

	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	s, err := store.New(sqlDB)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	hub := NewHub(s)
	srv := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	t.Cleanup(srv.Close)

	return &testServer{store: s, url: srv.URL}
}

type testClient struct {
	conn *websocket.Conn
}

func (ts *testServer) connect(t *testing.T, slug, token string) *testClient {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(ts.url, "http") + "?room=" + slug + "&token=" + token
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return &testClient{conn: conn}
}

func (c *testClient) send(t *testing.T, typ string, payload any) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	data, err := json.Marshal(envelope{Type: typ, Payload: body})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if err := c.conn.Write(context.Background(), websocket.MessageText, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// isPresence reports whether an event is someone arriving or leaving.
// Presence is broadcast to everyone already in the room whenever anyone
// opens or closes a connection, so for every test that isn't *about*
// presence it is background noise landing at an unhelpful moment — a
// second client connecting shifts the first client's stream by one.
func isPresence(eventType string) bool {
	return eventType == "participant.connected" || eventType == "participant.disconnected"
}

// readAnyEnvelope reads exactly the next envelope, whatever it is. Only
// the presence tests want this; everything else wants readEnvelope.
func (c *testClient) readAnyEnvelope(t *testing.T) envelope {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, data, err := c.conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	return env
}

// readEnvelope reads the next envelope that isn't presence chatter. A
// test about fog or tokens shouldn't have to know that a second client
// connecting announces itself to the first — that would make every
// multi-client test here a statement about presence as well as about
// its own feature, and it would break again the next time the connect
// sequence changed.
func (c *testClient) readEnvelope(t *testing.T) envelope {
	t.Helper()

	for {
		if env := c.readAnyEnvelope(t); !isPresence(env.Type) {
			return env
		}
	}
}

// readPresence is readEnvelope's mirror: the next arrival or departure,
// skipping everything else — notably the measure.ended that every
// disconnect also broadcasts.
func (c *testClient) readPresence(t *testing.T) envelope {
	t.Helper()

	for {
		if env := c.readAnyEnvelope(t); isPresence(env.Type) {
			return env
		}
	}
}

// expectNoMessage confirms nothing arrives within d — used to prove a
// recipient was deliberately skipped (e.g. a hidden token broadcast),
// not just that we haven't read far enough yet. Presence is ignored for
// the same reason readEnvelope ignores it; use expectNoPresence to
// assert on its absence.
//
// **This leaves the connection unusable.** coder/websocket tears down a
// connection whose read context is cancelled, so the deadline expiring
// — the success case here — closes it. Anything this is called on has
// to be finished with; for a "nothing happened" check in the middle of
// a test, send something and assert it comes straight back instead
// (assertNextIs in presence_test.go).
func (c *testClient) expectNoMessage(t *testing.T, d time.Duration) {
	t.Helper()

	deadline := time.Now().Add(d)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), remaining)
		_, data, err := c.conn.Read(ctx)
		cancel()
		if err != nil {
			return // nothing arrived before the deadline, which is the point
		}

		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			t.Fatalf("unmarshal envelope: %v", err)
		}
		if !isPresence(env.Type) {
			t.Fatalf("expected no message, but received %q", env.Type)
		}
	}
}

// expectNoPresence is the opposite assertion, for the presence tests
// themselves: nothing about anyone arriving or leaving may turn up,
// whatever else does.
func (c *testClient) expectNoPresence(t *testing.T, d time.Duration) {
	t.Helper()

	deadline := time.Now().Add(d)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), remaining)
		_, data, err := c.conn.Read(ctx)
		cancel()
		if err != nil {
			return
		}

		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			t.Fatalf("unmarshal envelope: %v", err)
		}
		if isPresence(env.Type) {
			t.Fatalf("expected no presence event, but received %q", env.Type)
		}
	}
}

func tokenCountFromScenePayload(t *testing.T, env envelope) int {
	t.Helper()

	var payload struct {
		Tokens []json.RawMessage `json:"tokens"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal %s payload: %v", env.Type, err)
	}
	return len(payload.Tokens)
}

func TestServeHTTP_RejectsMissingParams(t *testing.T) {
	ts := newTestServer(t)

	resp, err := http.Get(ts.url)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestServeHTTP_RejectsUnknownRoom(t *testing.T) {
	ts := newTestServer(t)

	resp, err := http.Get(ts.url + "?room=nosuchroom&token=garbage")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestServeHTTP_RejectsInvalidToken(t *testing.T) {
	ts := newTestServer(t)
	room, _, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	resp, err := http.Get(ts.url + "?room=" + room.Slug + "&token=garbage")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestStateSync_IncludesActiveSceneAndMessages(t *testing.T) {
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
	participantID := gm.ID
	if _, err := ts.store.InsertMessage(store.Message{
		RoomID: room.ID, ParticipantID: &participantID, ParticipantName: gm.DisplayName,
		Kind: store.MessageKindText, Body: "hi",
	}); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	client := ts.connect(t, room.Slug, gm.SessionToken)
	env := client.readEnvelope(t)
	if env.Type != "state.sync" {
		t.Fatalf("type = %q, want state.sync", env.Type)
	}

	var payload struct {
		Scene    *struct{ ID string `json:"id"` } `json:"scene"`
		Messages []json.RawMessage                `json:"messages"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal state.sync payload: %v", err)
	}
	if payload.Scene == nil || payload.Scene.ID != scene.ID {
		t.Fatalf("scene = %+v, want id %q", payload.Scene, scene.ID)
	}
	if len(payload.Messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(payload.Messages))
	}
}

func TestTokenCreate_NonGMRejected(t *testing.T) {
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

	client.send(t, "token.create", map[string]any{"sceneId": scene.ID, "name": "Goblin"})
	env := client.readEnvelope(t)
	if env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}

	tokens, err := ts.store.ListTokensForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListTokensForScene: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("len(tokens) = %d, want 0 (player must not be able to create tokens)", len(tokens))
	}
}

func TestTokenCreate_HiddenTokenOnlyBroadcastToGM(t *testing.T) {
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

	gmClient.send(t, "token.create", map[string]any{
		"sceneId": scene.ID, "name": "Hidden Goblin", "visibility": "hidden",
	})

	gmEnv := gmClient.readEnvelope(t)
	if gmEnv.Type != "token.created" {
		t.Fatalf("gm event type = %q, want token.created", gmEnv.Type)
	}

	playerClient.expectNoMessage(t, 200*time.Millisecond)
}

func TestSceneActivated_FiltersHiddenTokensForPlayers(t *testing.T) {
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
	if _, err := ts.store.CreateToken(store.Token{SceneID: scene.ID, Name: "Hidden", Visibility: store.VisibilityHidden}); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if _, err := ts.store.CreateToken(store.Token{SceneID: scene.ID, Name: "Visible", Visibility: store.VisibilityVisible}); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	gmClient := ts.connect(t, room.Slug, gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := ts.connect(t, room.Slug, player.SessionToken)
	playerClient.readEnvelope(t) // state.sync

	gmClient.send(t, "scene.setActive", map[string]any{"sceneId": scene.ID})

	gmEnv := gmClient.readEnvelope(t)
	playerEnv := playerClient.readEnvelope(t)

	if got := tokenCountFromScenePayload(t, gmEnv); got != 2 {
		t.Fatalf("gm sees %d tokens, want 2", got)
	}
	if got := tokenCountFromScenePayload(t, playerEnv); got != 1 {
		t.Fatalf("player sees %d tokens, want 1 (hidden token must be filtered)", got)
	}
}

func TestTokenMove_RejectsTokenFromAnotherRoom(t *testing.T) {
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
	token, err := ts.store.CreateToken(store.Token{SceneID: sceneA.ID, Name: "Goblin"})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	clientB := ts.connect(t, roomB.Slug, gmB.SessionToken)
	clientB.readEnvelope(t) // state.sync

	clientB.send(t, "token.move", map[string]any{"tokenId": token.ID, "x": 5, "y": 5})
	env := clientB.readEnvelope(t)
	if env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}

	tokens, err := ts.store.ListTokensForScene(sceneA.ID)
	if err != nil {
		t.Fatalf("ListTokensForScene: %v", err)
	}
	if tokens[0].X != 0 || tokens[0].Y != 0 {
		t.Fatalf("token position = (%v, %v), want unchanged (0, 0)", tokens[0].X, tokens[0].Y)
	}
}

func TestChatSend_SlashRollPersistsRollMessage(t *testing.T) {
	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	client := ts.connect(t, room.Slug, gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "chat.send", map[string]any{"text": "/roll 2d6+3"})
	env := client.readEnvelope(t)
	if env.Type != "chat.posted" {
		t.Fatalf("type = %q, want chat.posted", env.Type)
	}

	var payload struct {
		Kind          string  `json:"kind"`
		RollResult    *int    `json:"rollResult"`
		RollBreakdown *string `json:"rollBreakdown"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal chat.posted payload: %v", err)
	}
	if payload.Kind != "roll" {
		t.Fatalf("kind = %q, want roll", payload.Kind)
	}
	if payload.RollResult == nil {
		t.Fatal("expected a roll result")
	}
	if payload.RollBreakdown == nil || *payload.RollBreakdown == "" {
		t.Fatal("expected a non-empty roll breakdown")
	}
}

func TestChatSend_UnknownSlashCommandNotPersisted(t *testing.T) {
	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	client := ts.connect(t, room.Slug, gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "chat.send", map[string]any{"text": "/nonsense"})
	env := client.readEnvelope(t)
	if env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}

	messages, err := ts.store.ListRecentMessages(room.ID, 50)
	if err != nil {
		t.Fatalf("ListRecentMessages: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("len(messages) = %d, want 0 (unknown command must not be persisted)", len(messages))
	}
}

func TestHandleMessage_UnknownEnvelopeTypeReturnsError(t *testing.T) {
	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	client := ts.connect(t, room.Slug, gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "not.a.real.command", map[string]any{})
	env := client.readEnvelope(t)
	if env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
}
