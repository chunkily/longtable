package ws

import (
	"encoding/json"
	"testing"
	"time"

	"longtable/internal/store"
)

// The chat log's record of who came and went. Presence badges answer
// "who is here now" and heal themselves; these lines answer "who turned
// up tonight" and stay put, which is why they hang off the grace period
// rather than off the socket closing — a line written for a wobble on
// the wifi is still there an hour later.

// systemLines returns the room's own lines from its chat log, oldest
// first, as event/name pairs.
func systemLines(t *testing.T, ts *testServer, roomID string) [][2]string {
	t.Helper()

	messages, err := ts.store.ListRecentMessages(roomID, 50)
	if err != nil {
		t.Fatalf("ListRecentMessages: %v", err)
	}
	lines := make([][2]string, 0, len(messages))
	for i := len(messages) - 1; i >= 0; i-- { // the query is newest first
		if messages[i].Kind == store.MessageKindSystem {
			lines = append(lines, [2]string{messages[i].Body, messages[i].ParticipantName})
		}
	}
	return lines
}

// readSystemLine reads up to the next system chat message, which
// readEnvelope deliberately skips as presence noise.
func readSystemLine(t *testing.T, c *testClient) (event, name string) {
	t.Helper()

	for {
		env := c.readAnyEnvelope(t)
		if env.Type != "chat.posted" {
			continue
		}
		var msg struct {
			Kind            string `json:"kind"`
			Body            string `json:"body"`
			ParticipantName string `json:"participantName"`
		}
		if err := json.Unmarshal(env.Payload, &msg); err != nil {
			t.Fatalf("unmarshal chat.posted payload: %v", err)
		}
		if msg.Kind == string(store.MessageKindSystem) {
			return msg.Body, msg.ParticipantName
		}
	}
}

func TestSystemMessage_JoiningPutsALineInTheLogForEveryone(t *testing.T) {
	r := newTokenTestRoom(t)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	// Its own line first: unlike the presence badge, which skips the
	// client it is about, the log entry goes to everyone. The arrival's
	// own sync was sent before the line existed, so this is the only way
	// they get it without a refresh.
	if event, name := readSystemLine(t, gmClient); event != string(store.SystemEventJoined) || name != "GM" {
		t.Fatalf("first line = %q by %q, want the GM's own arrival", event, name)
	}

	r.ts.connect(t, r.room.Slug, r.player.SessionToken)

	// And then the player's, as the event rather than a sentence — the
	// wording belongs to the client.
	event, name := readSystemLine(t, gmClient)
	if event != string(store.SystemEventJoined) || name != "Bob" {
		t.Fatalf("line = %q by %q, want joined by Bob", event, name)
	}

	lines := systemLines(t, r.ts, r.room.ID)
	if len(lines) != 2 || lines[1] != [2]string{"joined", "Bob"} {
		t.Fatalf("log = %v, want the GM's arrival then Bob's", lines)
	}
}

// The durable half of the flicker fix. A blip has to leave the log
// exactly as it found it: unlike a badge, a line here doesn't heal.
func TestSystemMessage_AReconnectInsideTheGraceWritesNothing(t *testing.T) {
	r := newTokenTestRoom(t)

	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync
	before := systemLines(t, r.ts, r.room.ID)

	playerClient.conn.CloseNow()
	reconnected := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	reconnected.readEnvelope(t) // state.sync

	if after := systemLines(t, r.ts, r.room.ID); len(after) != len(before) {
		t.Fatalf("log = %v, want it unchanged from %v across a reconnect", after, before)
	}
}

func TestSystemMessage_LeavingPutsALineInTheLogOnceTheGraceExpires(t *testing.T) {
	r := newTokenTestRoom(t)
	grace := r.ts.hurryDepartures(50 * time.Millisecond)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t)    // state.sync
	readSystemLine(t, gmClient) // its own arrival

	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync
	if event, _ := readSystemLine(t, gmClient); event != string(store.SystemEventJoined) {
		t.Fatalf("line = %q, want the player's arrival first", event)
	}

	playerClient.conn.CloseNow()

	event, name := readSystemLine(t, gmClient)
	if event != string(store.SystemEventLeft) || name != "Bob" {
		t.Fatalf("line = %q by %q, want left by Bob", event, name)
	}

	// Stored, not merely broadcast: this is the half a late arrival and a
	// page refresh both read.
	time.Sleep(grace)
	lines := systemLines(t, r.ts, r.room.ID)
	if len(lines) == 0 || lines[len(lines)-1] != [2]string{"left", "Bob"} {
		t.Fatalf("log = %v, want it ending with Bob leaving", lines)
	}
}

// A system line is the room talking, so it carries no body a client
// could mistake for typed text, and names the participant it happened
// to.
func TestSystemMessage_CarriesTheEventAndTheNameRatherThanProse(t *testing.T) {
	r := newTokenTestRoom(t)

	// Read from a *second* client's sync rather than the first's: a
	// client's state.sync is sent before its own arrival is written, so
	// nobody's sync ever contains their own joined line. The player's
	// carries the GM's, which is the case a late arrival lands in.
	r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	env := client.readEnvelope(t) // state.sync

	var payload struct {
		Messages []struct {
			Kind            string `json:"kind"`
			Body            string `json:"body"`
			ParticipantName string `json:"participantName"`
			CreatedAt       string `json:"createdAt"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal state.sync payload: %v", err)
	}
	if len(payload.Messages) != 1 {
		t.Fatalf("synced messages = %+v, want just the GM's own arrival", payload.Messages)
	}

	line := payload.Messages[0]
	if line.Kind != string(store.MessageKindSystem) || line.Body != string(store.SystemEventJoined) {
		t.Fatalf("line = %+v, want a system line reading joined", line)
	}
	if line.ParticipantName != "GM" {
		t.Fatalf("participantName = %q, want GM", line.ParticipantName)
	}
	// The timestamp the chat panel renders. It has always been sent; this
	// pins it, since a system line is the one kind with nothing else in it.
	if _, err := time.Parse(time.RFC3339Nano, line.CreatedAt); err != nil {
		t.Fatalf("createdAt = %q, want an RFC3339 timestamp: %v", line.CreatedAt, err)
	}
}

// A seat removed while its owner is inside their grace window. The line
// still has to be written: it names the person, and the room wants to
// know they went. Found by watching the e2e log rather than by thinking
// about it — the foreign key refused the insert, and the only sign was
// one ERROR line in a run that otherwise passed.
func TestSystemMessage_SurvivesTheSeatBeingRemovedMidGrace(t *testing.T) {
	r := newTokenTestRoom(t)
	grace := r.ts.hurryDepartures(50 * time.Millisecond)

	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync
	playerClient.conn.CloseNow()

	if err := r.ts.store.DeleteSeat(r.room.ID, r.player.ID); err != nil {
		t.Fatalf("RemoveSeat: %v", err)
	}
	time.Sleep(grace * 4)

	lines := systemLines(t, r.ts, r.room.ID)
	if len(lines) == 0 || lines[len(lines)-1] != [2]string{"left", "Bob"} {
		t.Fatalf("log = %v, want it ending with Bob leaving", lines)
	}
}
