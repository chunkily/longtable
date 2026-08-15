package ws

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"longtable/internal/store"
)

// Presence is the one piece of room state with no row behind it: the
// roster is a table, but who is *connected* exists only in the hub's
// memory. These cover the seam between the two, and the fact that a
// person and a connection aren't the same thing.

type syncedPresence struct {
	Participants []struct {
		ID           string `json:"id"`
		DisplayName  string `json:"displayName"`
		Role         string `json:"role"`
		SessionToken string `json:"sessionToken"`
	} `json:"participants"`
	ConnectedParticipantIDs []string `json:"connectedParticipantIds"`
}

func presenceFromSync(t *testing.T, env envelope) syncedPresence {
	t.Helper()

	if env.Type != "state.sync" {
		t.Fatalf("type = %q, want state.sync", env.Type)
	}
	var payload syncedPresence
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal state.sync payload: %v", err)
	}
	return payload
}

func connectedID(t *testing.T, env envelope) string {
	t.Helper()

	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal participant.connected payload: %v", err)
	}
	return payload.ID
}

func disconnectedID(t *testing.T, env envelope) string {
	t.Helper()

	var payload struct {
		ParticipantID string `json:"participantId"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal participant.disconnected payload: %v", err)
	}
	return payload.ParticipantID
}

// assertNoPresenceYet pins "nobody was announced" without a timeout, by
// sending a chat message and reading up to its own echo: anything the
// server broadcast earlier is queued ahead of that echo, so a presence
// event that shouldn't exist arrives first and fails here.
//
// A timeout would be simpler but the read-with-deadline helpers close
// the connection they give up on, and these clients have more to do.
// Unrelated traffic is allowed past — every disconnect also broadcasts
// a measure.ended, unconditionally and by design.
func assertNoPresenceYet(t *testing.T, c *testClient) {
	t.Helper()

	c.send(t, "chat.send", map[string]any{"text": "sync point"})
	for {
		env := c.readAnyEnvelope(t)
		if env.Type == "chat.posted" {
			return
		}
		if isPresence(env.Type) {
			t.Fatalf("presence event %q arrived when nobody had come or gone", env.Type)
		}
	}
}

func TestStateSync_CarriesTheRosterAndWhoIsConnected(t *testing.T) {
	r := newTokenTestRoom(t)

	// Someone who joined and never came back is on the roster and not in
	// the connected list — the whole reason the two are separate.
	absent, err := r.ts.store.JoinRoom(r.room.ID, "Carol", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	presence := presenceFromSync(t, gmClient.readEnvelope(t))

	names := make([]string, 0, len(presence.Participants))
	for _, p := range presence.Participants {
		names = append(names, p.DisplayName)
	}
	if len(presence.Participants) != 3 {
		t.Fatalf("roster = %v, want all three who have ever joined", names)
	}
	if len(presence.ConnectedParticipantIDs) != 1 || presence.ConnectedParticipantIDs[0] != r.gm.ID {
		t.Fatalf("connected = %v, want just the GM", presence.ConnectedParticipantIDs)
	}
	for _, p := range presence.Participants {
		if p.ID == absent.ID {
			return // on the roster, correctly absent from the connected list
		}
	}
	t.Fatalf("roster %v is missing the participant who isn't connected", names)
}

// A session token is a credential. The roster goes to everyone in the
// room, so one leaking would hand every Player the GM's login.
func TestStateSync_RosterNeverCarriesSessionTokens(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	env := client.readEnvelope(t)

	for _, p := range presenceFromSync(t, env).Participants {
		if p.SessionToken != "" {
			t.Fatalf("participant %q carried a session token", p.DisplayName)
		}
	}
	// Belt and braces: the GM's token must not appear anywhere in the
	// payload, under any key this test didn't think to name.
	if strings.Contains(string(env.Payload), r.gm.SessionToken) {
		t.Fatal("state.sync payload contains the GM's session token")
	}
}

func TestPresence_ArrivalAndDepartureReachTheRoom(t *testing.T) {
	r := newTokenTestRoom(t)
	// A departure is announced by a timer now, not by the disconnect.
	r.ts.hurryDepartures(50 * time.Millisecond)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync

	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync

	env := gmClient.readPresence(t)
	if env.Type != "participant.connected" {
		t.Fatalf("type = %q, want participant.connected", env.Type)
	}
	if got := connectedID(t, env); got != r.player.ID {
		t.Fatalf("connected id = %q, want the player's %q", got, r.player.ID)
	}

	playerClient.conn.CloseNow()

	env = gmClient.readPresence(t)
	if env.Type != "participant.disconnected" {
		t.Fatalf("type = %q, want participant.disconnected", env.Type)
	}
	if got := disconnectedID(t, env); got != r.player.ID {
		t.Fatalf("disconnected id = %q, want the player's %q", got, r.player.ID)
	}
}

// The arriving client learns it is connected from its own state.sync,
// not from an echo of its own arrival. This is the one broadcast that
// skips its sender, so it is worth pinning: an echo here would be a
// second copy of something the client acted on a moment earlier, and
// every test that opens a connection would have to read past it.
func TestPresence_ArrivalIsInTheNewcomersSyncRatherThanEchoedBack(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	presence := presenceFromSync(t, client.readEnvelope(t))

	if len(presence.ConnectedParticipantIDs) != 1 || presence.ConnectedParticipantIDs[0] != r.gm.ID {
		t.Fatalf("connected = %v, want the newcomer to see itself", presence.ConnectedParticipantIDs)
	}
	client.expectNoPresence(t, 200*time.Millisecond)
}

// A second browser tab is a second connection but the same person. It
// must not announce an arrival, and closing it must not announce a
// departure while the first tab is still looking at the map.
func TestPresence_ASecondTabIsNotASecondPerson(t *testing.T) {
	r := newTokenTestRoom(t)
	// The last tab closing still only announces once the grace period is
	// up, so shorten it rather than waiting half a minute for the end of
	// this test.
	r.ts.hurryDepartures(50 * time.Millisecond)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync

	firstTab := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	firstTab.readEnvelope(t) // state.sync
	if env := gmClient.readPresence(t); env.Type != "participant.connected" {
		t.Fatalf("type = %q, want participant.connected for the first tab", env.Type)
	}

	// The second tab's own sync still lists the player once, in both the
	// roster and the connected list.
	secondTab := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	presence := presenceFromSync(t, secondTab.readEnvelope(t))
	seen := 0
	for _, id := range presence.ConnectedParticipantIDs {
		if id == r.player.ID {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("player appears %d times in %v, want once", seen, presence.ConnectedParticipantIDs)
	}

	// Nothing for the GM: the player was already here. Proved with a
	// round trip rather than a timeout — a chat echo can only arrive
	// after anything the connection wrongly broadcast, so if the second
	// tab announced itself that lands first and this reads it. A timeout
	// would work too, but the read-with-deadline helpers close the
	// connection they give up on, and this client has more to do.
	assertNoPresenceYet(t, gmClient)

	secondTab.conn.CloseNow()
	// Still nothing: one tab closing leaves the person at the table. This
	// is the half a "then assert the next presence event" check can't
	// make on its own — a spurious departure here is indistinguishable
	// from the real one that follows.
	assertNoPresenceYet(t, gmClient)

	firstTab.conn.CloseNow()
	env := gmClient.readPresence(t)
	if env.Type != "participant.disconnected" {
		t.Fatalf("type = %q, want participant.disconnected once the last tab went", env.Type)
	}
	if got := disconnectedID(t, env); got != r.player.ID {
		t.Fatalf("disconnected id = %q, want %q", got, r.player.ID)
	}
}

func TestListParticipantsForRoom_IsScopedToItsRoomAndOrderedByJoin(t *testing.T) {
	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	player, err := ts.store.JoinRoom(room.ID, "Bob", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	other, _, err := ts.store.CreateRoom("Other", "Their GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if _, err := ts.store.JoinRoom(other.ID, "Stranger", ""); err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	participants, err := ts.store.ListParticipantsForRoom(room.ID)
	if err != nil {
		t.Fatalf("ListParticipantsForRoom: %v", err)
	}
	if len(participants) != 2 {
		t.Fatalf("got %d participants, want 2 — another room's members must not appear", len(participants))
	}
	if participants[0].ID != gm.ID || participants[1].ID != player.ID {
		t.Fatalf("order = %q, %q, want the GM first", participants[0].DisplayName, participants[1].DisplayName)
	}
	if participants[0].SessionToken != "" {
		t.Fatal("the roster query loaded a session token, which it must never do")
	}
	if participants[0].Role != store.RoleGM || participants[1].Role != store.RolePlayer {
		t.Fatalf("roles = %q, %q", participants[0].Role, participants[1].Role)
	}
}

// The flicker this grace period exists for. A dropped socket used to
// take the badge off every other rail in the room and the reconnect put
// it back, half a second later at best and the better part of a minute
// at worst as the client's backoff doubled. Nothing about that is a fact
// anyone at the table wanted to know.
//
// The assertion is the strong one: not "it came back" but "nothing was
// ever said". A resumption that announced an arrival would still leave
// the badge correct, and would still write "Bob left / Bob joined" into
// the chat log for a wobble on the wifi.
func TestPresence_AReconnectInsideTheGraceSaysNothingAtAll(t *testing.T) {
	r := newTokenTestRoom(t)
	// The production grace, deliberately: the point is that the reconnect
	// lands well inside it.

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync

	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync
	if env := gmClient.readPresence(t); env.Type != "participant.connected" {
		t.Fatalf("type = %q, want the player's arrival", env.Type)
	}

	playerClient.conn.CloseNow()
	reconnected := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	reconnected.readEnvelope(t) // state.sync

	// Neither a departure nor a second arrival, proved with a round trip
	// rather than a timeout so this connection stays usable.
	assertNoPresenceYet(t, gmClient)

	// And the room still agrees the player is here.
	presence := presenceFromSync(t, r.ts.connect(t, r.room.Slug, r.gm.SessionToken).readEnvelope(t))
	if !slices.Contains(presence.ConnectedParticipantIDs, r.player.ID) {
		t.Fatalf("connected = %v, want the reconnected player among them", presence.ConnectedParticipantIDs)
	}
}

// The trap that makes the grace period worth testing separately from the
// flicker: a client syncing *during* someone else's grace window has to
// see the same room as everyone else. A resumption is silent by design,
// so if this sync left the departing participant out, the arrival that
// would have corrected it never comes — and one rail is quietly missing
// a person until something else resyncs it.
func TestPresence_SomeoneInsideTheirGraceWindowIsStillConnected(t *testing.T) {
	r := newTokenTestRoom(t)

	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync
	playerClient.conn.CloseNow()

	// Fresh client, mid-window: the player's grace has not expired.
	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	presence := presenceFromSync(t, gmClient.readEnvelope(t))
	if !slices.Contains(presence.ConnectedParticipantIDs, r.player.ID) {
		t.Fatalf("connected = %v, want the departing player still counted", presence.ConnectedParticipantIDs)
	}
}
