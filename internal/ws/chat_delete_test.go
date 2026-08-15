package ws

import (
	"encoding/json"
	"testing"
)

// chatDeleteTestRoom sets up a room with a GM and a Player, and posts
// one chat message from the Player — the shared fixture for the delete
// and purge tests below.
type chatDeleteTestRoom struct {
	ts   *testServer
	room string // slug
	gm   string // session token
	bob  string // session token
	msg  string // message id
	body string // the message's original text
}

func newChatDeleteTestRoom(t *testing.T) chatDeleteTestRoom {
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

	const body = "hello"
	client := ts.connect(t, room.Slug, player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "chat.send", map[string]any{"text": body})
	env := client.readEnvelope(t)
	if env.Type != "chat.posted" {
		t.Fatalf("type = %q, want chat.posted", env.Type)
	}
	var posted struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(env.Payload, &posted); err != nil {
		t.Fatalf("unmarshal chat.posted payload: %v", err)
	}

	return chatDeleteTestRoom{ts: ts, room: room.Slug, gm: gm.SessionToken, bob: player.SessionToken, msg: posted.ID, body: body}
}

func decodeChatMessagePayload(t *testing.T, env envelope) struct {
	ID   string `json:"id"`
	Body string `json:"body"`
} {
	t.Helper()
	var payload struct {
		ID   string `json:"id"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal message payload: %v", err)
	}
	return payload
}

// The author of a message is one of the two people still allowed to see
// its original content once it's deleted, alongside whoever deleted it.
func TestChatDelete_AuthorSeesStrikethroughContentAfterOwnDelete(t *testing.T) {
	r := newChatDeleteTestRoom(t)

	client := r.ts.connect(t, r.room, r.bob)
	client.readEnvelope(t) // state.sync

	client.send(t, "chat.delete", map[string]any{"messageId": r.msg})
	env := client.readEnvelope(t)
	if env.Type != "chat.deleted" {
		t.Fatalf("type = %q, want chat.deleted", env.Type)
	}
	payload := decodeChatMessagePayload(t, env)
	if payload.ID != r.msg {
		t.Fatalf("id = %q, want %q", payload.ID, r.msg)
	}
	if payload.Body != r.body {
		t.Fatalf("body = %q, want the original %q (author should still see it)", payload.Body, r.body)
	}

	// And the row itself keeps the content — redaction is per viewer at
	// broadcast time, not something done to the row.
	msg, err := r.ts.store.GetMessage(r.msg)
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	if msg.Body != r.body {
		t.Fatalf("stored Body = %q, want %q kept", msg.Body, r.body)
	}
	if msg.DeletedAt == nil {
		t.Fatal("DeletedAt = nil, want set after soft-delete")
	}
}

// A third participant — neither the author nor the one who deleted it —
// gets the generic placeholder, never the original text.
func TestChatDelete_OtherParticipantsSeeOnlyThePlaceholder(t *testing.T) {
	r := newChatDeleteTestRoom(t)

	eve, err := r.ts.store.JoinRoom(mustRoomIDFromSlug(t, r.ts, r.room), "Eve")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	eveClient := r.ts.connect(t, r.room, eve.SessionToken)
	eveClient.readEnvelope(t) // state.sync

	bobClient := r.ts.connect(t, r.room, r.bob)
	bobClient.readEnvelope(t) // state.sync

	bobClient.send(t, "chat.delete", map[string]any{"messageId": r.msg})
	if env := bobClient.readEnvelope(t); env.Type != "chat.deleted" {
		t.Fatalf("author: type = %q, want chat.deleted", env.Type)
	}

	env := eveClient.readEnvelope(t)
	if env.Type != "chat.deleted" {
		t.Fatalf("eve: type = %q, want chat.deleted", env.Type)
	}
	payload := decodeChatMessagePayload(t, env)
	if payload.Body != "" {
		t.Fatalf("eve's body = %q, want empty (she's neither author nor deleter)", payload.Body)
	}
}

// A GM moderating someone else's message is a case where the author and
// the deleter differ: both should still see the content, and it should
// still be gone for a bystander.
func TestChatDelete_GMDeletingSomeoneElsesMessage_BothSeeContentBystanderDoesNot(t *testing.T) {
	r := newChatDeleteTestRoom(t)

	eve, err := r.ts.store.JoinRoom(mustRoomIDFromSlug(t, r.ts, r.room), "Eve")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	eveClient := r.ts.connect(t, r.room, eve.SessionToken)
	eveClient.readEnvelope(t)                   // state.sync
	bobClient := r.ts.connect(t, r.room, r.bob) // the author
	bobClient.readEnvelope(t)                   // state.sync
	gmClient := r.ts.connect(t, r.room, r.gm)   // the deleter
	gmClient.readEnvelope(t)                    // state.sync

	gmClient.send(t, "chat.delete", map[string]any{"messageId": r.msg})

	gmEnv := gmClient.readEnvelope(t)
	if payload := decodeChatMessagePayload(t, gmEnv); payload.Body != r.body {
		t.Fatalf("gm (deleter) body = %q, want %q", payload.Body, r.body)
	}
	bobEnv := bobClient.readEnvelope(t)
	if payload := decodeChatMessagePayload(t, bobEnv); payload.Body != r.body {
		t.Fatalf("bob (author) body = %q, want %q", payload.Body, r.body)
	}
	eveEnv := eveClient.readEnvelope(t)
	if payload := decodeChatMessagePayload(t, eveEnv); payload.Body != "" {
		t.Fatalf("eve (bystander) body = %q, want empty", payload.Body)
	}

	msg, err := r.ts.store.GetMessage(r.msg)
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	if msg.DeletedByParticipantID == nil {
		t.Fatal("DeletedByParticipantID = nil, want the GM's participant id")
	}
}

// A client connecting after the first delete has already happened gets
// the same split via state.sync as one that was there live for it.
func TestStateSync_ReflectsChatDeleteRedactionPerViewer(t *testing.T) {
	r := newChatDeleteTestRoom(t)

	bobClient := r.ts.connect(t, r.room, r.bob)
	bobClient.readEnvelope(t) // state.sync
	bobClient.send(t, "chat.delete", map[string]any{"messageId": r.msg})
	bobClient.readEnvelope(t) // chat.deleted

	// Bob (the author) reconnecting still sees the content in his sync.
	bobClient2 := r.ts.connect(t, r.room, r.bob)
	env := bobClient2.readEnvelope(t)
	if env.Type != "state.sync" {
		t.Fatalf("type = %q, want state.sync", env.Type)
	}
	// Filtered to what people said: the sync also carries the room's own
	// joined lines, one per connection this test has opened.
	said := saidInSync(t, env)
	if len(said) != 1 || said[0].Body != r.body {
		t.Fatalf("bob's synced messages = %+v, want the original body kept", said)
	}

	// A stranger connecting fresh only ever sees the placeholder.
	eve, err := r.ts.store.JoinRoom(mustRoomIDFromSlug(t, r.ts, r.room), "Eve")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	eveClient := r.ts.connect(t, r.room, eve.SessionToken)
	eveSaid := saidInSync(t, eveClient.readEnvelope(t))
	if len(eveSaid) != 1 || eveSaid[0].Body != "" {
		t.Fatalf("eve's synced messages = %+v, want redacted", eveSaid)
	}
}

func TestChatDelete_SecondCallPurgesEntirely(t *testing.T) {
	r := newChatDeleteTestRoom(t)

	client := r.ts.connect(t, r.room, r.bob)
	client.readEnvelope(t) // state.sync

	client.send(t, "chat.delete", map[string]any{"messageId": r.msg})
	if env := client.readEnvelope(t); env.Type != "chat.deleted" {
		t.Fatalf("first delete: type = %q, want chat.deleted", env.Type)
	}

	client.send(t, "chat.delete", map[string]any{"messageId": r.msg})
	env := client.readEnvelope(t)
	if env.Type != "chat.purged" {
		t.Fatalf("second delete: type = %q, want chat.purged", env.Type)
	}
	var payload struct {
		MessageID string `json:"messageId"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal chat.purged payload: %v", err)
	}
	if payload.MessageID != r.msg {
		t.Fatalf("messageId = %q, want %q", payload.MessageID, r.msg)
	}

	if _, err := r.ts.store.GetMessage(r.msg); err == nil {
		t.Fatal("GetMessage after purge: want error, got nil (row should be gone)")
	}
}

func TestChatDelete_GMDeletesAnyonesMessage(t *testing.T) {
	r := newChatDeleteTestRoom(t)

	client := r.ts.connect(t, r.room, r.gm)
	client.readEnvelope(t) // state.sync

	client.send(t, "chat.delete", map[string]any{"messageId": r.msg})
	if env := client.readEnvelope(t); env.Type != "chat.deleted" {
		t.Fatalf("type = %q, want chat.deleted", env.Type)
	}

	// And the GM can purge it too, same as any Player could with their
	// own message.
	client.send(t, "chat.delete", map[string]any{"messageId": r.msg})
	if env := client.readEnvelope(t); env.Type != "chat.purged" {
		t.Fatalf("type = %q, want chat.purged", env.Type)
	}
}

func TestChatDelete_PlayerCannotDeleteSomeoneElsesMessage(t *testing.T) {
	r := newChatDeleteTestRoom(t)

	other, err := r.ts.store.JoinRoom(mustRoomIDFromSlug(t, r.ts, r.room), "Eve")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	client := r.ts.connect(t, r.room, other.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "chat.delete", map[string]any{"messageId": r.msg})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}

	msg, err := r.ts.store.GetMessage(r.msg)
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	if msg.DeletedAt != nil {
		t.Fatal("message should not have been touched")
	}
}

func TestChatDelete_RejectsMessageFromAnotherRoom(t *testing.T) {
	r := newChatDeleteTestRoom(t)

	otherRoom, otherGM, err := r.ts.store.CreateRoom("Room B", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	client := r.ts.connect(t, otherRoom.Slug, otherGM.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "chat.delete", map[string]any{"messageId": r.msg})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}

	msg, err := r.ts.store.GetMessage(r.msg)
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	if msg.DeletedAt != nil {
		t.Fatal("message in the other room should not have been touched")
	}
}

func TestChatDelete_RejectsUnknownMessage(t *testing.T) {
	r := newChatDeleteTestRoom(t)

	client := r.ts.connect(t, r.room, r.gm)
	client.readEnvelope(t) // state.sync

	client.send(t, "chat.delete", map[string]any{"messageId": "nope"})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
}

func mustRoomIDFromSlug(t *testing.T, ts *testServer, slug string) string {
	t.Helper()
	room, err := ts.store.GetRoomBySlug(slug)
	if err != nil {
		t.Fatalf("GetRoomBySlug: %v", err)
	}
	return room.ID
}
