package ws

import (
	"encoding/json"
	"testing"
	"time"

	"longtable/internal/store"
)

// The turn order. What needs a real socket here is the filtering — a
// Player must not learn that a hidden combatant exists — and the turn
// arithmetic across a round boundary, which is where a table starts
// arguing about whether a spell has expired.

type initiativePayloadShape struct {
	Entries []struct {
		ID           string  `json:"id"`
		TokenID      *string `json:"tokenId"`
		Name         string  `json:"name"`
		Initiative   float64 `json:"initiative"`
		Hidden       bool    `json:"hidden"`
		ImageAssetID *string `json:"imageAssetId"`
	} `json:"entries"`
	Round          int     `json:"round"`
	CurrentEntryID *string `json:"currentEntryId"`
}

func readInitiative(t *testing.T, c *testClient) initiativePayloadShape {
	t.Helper()

	env := c.readEnvelope(t)
	if env.Type != "initiative.updated" {
		t.Fatalf("type = %q, want initiative.updated", env.Type)
	}
	var payload initiativePayloadShape
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal initiative.updated payload: %v", err)
	}
	return payload
}

func (p initiativePayloadShape) names() []string {
	out := make([]string, 0, len(p.Entries))
	for _, e := range p.Entries {
		out = append(out, e.Name)
	}
	return out
}

func namesEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// The order is the initiative values, highest first — a tracker that
// listed combatants in the order they were typed would be a list, not a
// tracker.
func TestInitiativeAdd_SortsByValueHighestFirst(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	for _, entry := range []struct {
		name  string
		value float64
	}{{"Goblin", 12}, {"Bob", 19}, {"Lair action", 20}} {
		client.send(t, "initiative.add", map[string]any{"name": entry.name, "initiative": entry.value})
		readInitiative(t, client)
	}

	client.send(t, "initiative.add", map[string]any{"name": "Wolf", "initiative": 15})
	got := readInitiative(t, client).names()
	if want := []string{"Lair action", "Bob", "Wolf", "Goblin"}; !namesEqual(got, want) {
		t.Fatalf("order = %v, want %v — a new entry has to land in its sorted place", got, want)
	}
}

// A linked entry takes its name from the token every time it's sent,
// rather than copying it once: renaming a token has to rename its entry.
func TestInitiativeAdd_LinkedEntryFollowsItsToken(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "initiative.add", map[string]any{"tokenId": token.ID, "initiative": 14})
	entries := readInitiative(t, client).Entries
	if len(entries) != 1 || entries[0].Name != "Goblin" {
		t.Fatalf("entries = %+v, want one called Goblin", entries)
	}

	client.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "name": "Hobgoblin", "width": 2, "height": 2, "visibility": "visible",
	})
	if env := client.readEnvelope(t); env.Type != "token.updated" {
		t.Fatalf("type = %q, want token.updated", env.Type)
	}

	// The rename reaches the tracker without anyone having touched it.
	renamed := readInitiative(t, client)
	if renamed.Entries[0].Name != "Hobgoblin" {
		t.Fatalf("entry name = %q, want it to follow the token", renamed.Entries[0].Name)
	}
}

// Two ways to be invisible to a Player, and the tracker has to obey
// both — one the GM set on the entry, one the token brought with it.
func TestInitiative_HiddenEntriesAreWithheldFromPlayers(t *testing.T) {
	r := newTokenTestRoom(t)
	ambusher := r.token(t, "Ambusher", store.VisibilityHidden)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync
	gmClient.readPresence(t)     // the player arriving

	gmClient.send(t, "initiative.add", map[string]any{"name": "Bob", "initiative": 10})
	readInitiative(t, gmClient)
	readInitiative(t, playerClient)

	gmClient.send(t, "initiative.add", map[string]any{
		"name": "Something in the dark", "initiative": 18, "hidden": true,
	})
	readInitiative(t, gmClient)
	readInitiative(t, playerClient)

	gmClient.send(t, "initiative.add", map[string]any{"tokenId": ambusher.ID, "initiative": 25})

	gm := readInitiative(t, gmClient)
	if want := []string{"Ambusher", "Something in the dark", "Bob"}; !namesEqual(gm.names(), want) {
		t.Fatalf("gm sees %v, want %v", gm.names(), want)
	}

	// Not filtered out, not nulled: never sent. A Player counting entries
	// would otherwise know how many things are waiting in the dark.
	player := readInitiative(t, playerClient)
	if want := []string{"Bob"}; !namesEqual(player.names(), want) {
		t.Fatalf("player sees %v, want %v", player.names(), want)
	}
}

// Revealing a token has to put its entry back on a Player's tracker,
// which nothing in the tracker itself has any reason to notice.
func TestInitiative_RevealingATokenAddsItsEntryForPlayers(t *testing.T) {
	r := newTokenTestRoom(t)
	ambusher := r.token(t, "Ambusher", store.VisibilityHidden)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync
	gmClient.readPresence(t)     // the player arriving

	gmClient.send(t, "initiative.add", map[string]any{"tokenId": ambusher.ID, "initiative": 25})
	readInitiative(t, gmClient)
	if got := readInitiative(t, playerClient).names(); len(got) != 0 {
		t.Fatalf("player sees %v before the reveal, want nothing", got)
	}

	gmClient.send(t, "token.update", map[string]any{
		"tokenId": ambusher.ID, "name": "Ambusher", "width": 2, "height": 2, "visibility": "visible",
	})
	if env := gmClient.readEnvelope(t); env.Type != "token.updated" {
		t.Fatalf("type = %q, want token.updated", env.Type)
	}
	readInitiative(t, gmClient)

	playerClient.readEnvelope(t) // token.updated — the token itself arriving
	if got := readInitiative(t, playerClient).names(); !namesEqual(got, []string{"Ambusher"}) {
		t.Fatalf("player sees %v after the reveal, want [Ambusher]", got)
	}
}

// The round changes only at the wrap, in both directions — so next then
// previous lands exactly where it started, across a round boundary too.
func TestInitiativeAdvance_WrapsAndCountsRounds(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	for _, name := range []string{"First", "Second"} {
		value := 20.0
		if name == "Second" {
			value = 10
		}
		client.send(t, "initiative.add", map[string]any{"name": name, "initiative": value})
		readInitiative(t, client)
	}

	// The first press starts the encounter rather than counting a round
	// nobody has played.
	client.send(t, "initiative.advance", map[string]any{"direction": "next"})
	state := readInitiative(t, client)
	if state.CurrentEntryID == nil || *state.CurrentEntryID != state.Entries[0].ID || state.Round != 1 {
		t.Fatalf("after the first Next: current = %v, round = %d, want the top of round 1", state.CurrentEntryID, state.Round)
	}

	client.send(t, "initiative.advance", map[string]any{"direction": "next"})
	state = readInitiative(t, client)
	if *state.CurrentEntryID != state.Entries[1].ID || state.Round != 1 {
		t.Fatalf("second turn should still be round 1, got round %d", state.Round)
	}

	client.send(t, "initiative.advance", map[string]any{"direction": "next"})
	state = readInitiative(t, client)
	if *state.CurrentEntryID != state.Entries[0].ID || state.Round != 2 {
		t.Fatalf("wrapping should start round 2, got round %d", state.Round)
	}

	// And back again — the one that would be off by one if the round were
	// changed anywhere but at the wrap.
	client.send(t, "initiative.advance", map[string]any{"direction": "previous"})
	state = readInitiative(t, client)
	if *state.CurrentEntryID != state.Entries[1].ID || state.Round != 1 {
		t.Fatalf("stepping back over the wrap = entry %v round %d, want the last entry of round 1", *state.CurrentEntryID, state.Round)
	}
}

// Round 1 is the floor: going back before the first turn of the first
// round would be a fight that hasn't happened.
func TestInitiativeAdvance_NeverGoesBelowRoundOne(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "initiative.add", map[string]any{"name": "Only", "initiative": 10})
	readInitiative(t, client)

	for range 3 {
		client.send(t, "initiative.advance", map[string]any{"direction": "previous"})
		if got := readInitiative(t, client).Round; got < 1 {
			t.Fatalf("round = %d, want never below 1", got)
		}
	}
}

func TestInitiativeAdvance_RefusesAnEmptyOrder(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "initiative.advance", map[string]any{"direction": "next"})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
}

// Removing whoever is up hands the turn on rather than leaving the
// tracker pointing at a corpse.
func TestInitiativeRemove_HandsOnTheTurnItWasHolding(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	for _, name := range []string{"First", "Second"} {
		value := 20.0
		if name == "Second" {
			value = 10
		}
		client.send(t, "initiative.add", map[string]any{"name": name, "initiative": value})
		readInitiative(t, client)
	}

	client.send(t, "initiative.advance", map[string]any{"direction": "next"})
	state := readInitiative(t, client)
	current := *state.CurrentEntryID

	client.send(t, "initiative.remove", map[string]any{"entryId": current})
	after := readInitiative(t, client)
	if len(after.Entries) != 1 || after.Entries[0].Name != "Second" {
		t.Fatalf("entries = %v, want just Second", after.names())
	}
	if after.CurrentEntryID == nil || *after.CurrentEntryID != after.Entries[0].ID {
		t.Fatalf("current = %v, want the turn handed to Second", after.CurrentEntryID)
	}
}

// Deleting the token takes its entry with it — the cascade is the
// mechanism, and being told about it is the part that needs a hub.
func TestInitiative_DeletingATokenRemovesItsEntry(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "initiative.add", map[string]any{"tokenId": token.ID, "initiative": 14})
	readInitiative(t, client)

	client.send(t, "token.delete", map[string]any{"tokenId": token.ID})
	if env := client.readEnvelope(t); env.Type != "token.deleted" {
		t.Fatalf("type = %q, want token.deleted", env.Type)
	}
	if got := readInitiative(t, client).names(); len(got) != 0 {
		t.Fatalf("entries = %v, want the entry gone with its token", got)
	}
}

// Ties are the case a manual order exists for; anything else would let
// the list disagree with the numbers printed beside it.
func TestInitiativeReorder_MovesWithinATieAndRefusesOtherwise(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	// The last add's broadcast is the whole tracker, so it carries the
	// ids the reorder below needs.
	var entries initiativePayloadShape
	for _, name := range []string{"Alice", "Bob", "Slow"} {
		value := 15.0
		if name == "Slow" {
			value = 5
		}
		client.send(t, "initiative.add", map[string]any{"name": name, "initiative": value})
		entries = readInitiative(t, client)
	}
	if !namesEqual(entries.names(), []string{"Alice", "Bob", "Slow"}) {
		t.Fatalf("starting order = %v", entries.names())
	}

	// Bob argues that he was ready first.
	client.send(t, "initiative.reorder", map[string]any{"entryId": entries.Entries[1].ID, "direction": "up"})
	after := readInitiative(t, client)
	if want := []string{"Bob", "Alice", "Slow"}; !namesEqual(after.names(), want) {
		t.Fatalf("order = %v, want %v", after.names(), want)
	}

	// Alice can't be pushed below Slow: that would put a 15 under a 5.
	client.send(t, "initiative.reorder", map[string]any{"entryId": after.Entries[1].ID, "direction": "down"})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error — reordering across values must be refused", env.Type)
	}
}

func TestInitiativeClear_EmptiesTheOrderAndResetsTheRound(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "initiative.add", map[string]any{"tokenId": token.ID, "initiative": 14})
	readInitiative(t, client)
	client.send(t, "initiative.advance", map[string]any{"direction": "next"})
	client.send(t, "initiative.advance", map[string]any{"direction": "next"})
	readInitiative(t, client)
	if got := readInitiative(t, client).Round; got != 2 {
		t.Fatalf("round = %d, want 2 before clearing", got)
	}

	client.send(t, "initiative.clear", map[string]any{})
	cleared := readInitiative(t, client)
	if len(cleared.Entries) != 0 || cleared.Round != 1 || cleared.CurrentEntryID != nil {
		t.Fatalf("cleared = %+v, want an empty order back at round 1 with nobody up", cleared)
	}

	// The token is still on the map: leaving the order is not leaving the
	// map, which is the whole difference between clearing and deleting.
	if _, err := r.ts.store.GetToken(token.ID); err != nil {
		t.Fatalf("GetToken after clear: %v", err)
	}
}

// Every one of these is the GM's, and a Player must not be able to
// rearrange the fight from the console.
func TestInitiative_PlayerMayNotChangeAnything(t *testing.T) {
	r := newTokenTestRoom(t)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	gmClient.send(t, "initiative.add", map[string]any{"name": "Bob", "initiative": 10})
	entry := readInitiative(t, gmClient).Entries[0]

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	for name, payload := range map[string]map[string]any{
		"add":     {"name": "Mine", "initiative": 99},
		"update":  {"entryId": entry.ID, "name": "Mine", "initiative": 99},
		"remove":  {"entryId": entry.ID},
		"reorder": {"entryId": entry.ID, "direction": "up"},
		"advance": {"direction": "next"},
		"clear":   {},
	} {
		client.send(t, "initiative."+name, payload)
		if env := client.readEnvelope(t); env.Type != "error" {
			t.Fatalf("initiative.%s: type = %q, want error", name, env.Type)
		}
	}

	client.expectNoMessage(t, 200*time.Millisecond)
}

// An entry id from another room answers exactly as one that doesn't
// exist, so the tracker can't be used to probe for what's elsewhere.
func TestInitiative_EntryFromAnotherRoomFailsLikeAMissingOne(t *testing.T) {
	r := newTokenTestRoom(t)

	other, otherGM, err := r.ts.store.CreateRoom("Other", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	entry, err := r.ts.store.CreateInitiativeEntry(store.InitiativeEntry{
		RoomID: other.ID, Name: "Elsewhere", Initiative: 10,
	})
	if err != nil {
		t.Fatalf("CreateInitiativeEntry: %v", err)
	}
	_ = otherGM

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "initiative.remove", map[string]any{"entryId": entry.ID})
	fromOtherRoom := errorMessage(t, client.readEnvelope(t))

	client.send(t, "initiative.remove", map[string]any{
		"entryId": "00000000-0000-4000-8000-000000000000",
	})
	fromMissing := errorMessage(t, client.readEnvelope(t))

	if fromOtherRoom != fromMissing {
		t.Fatalf("errors differ: %q vs %q — the two must be indistinguishable", fromOtherRoom, fromMissing)
	}
}

// A client that connects mid-encounter has to arrive knowing the order,
// whose turn it is and the round — the tracker isn't replayed from
// events.
func TestStateSync_CarriesTheInitiativeTracker(t *testing.T) {
	r := newTokenTestRoom(t)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	gmClient.send(t, "initiative.add", map[string]any{"name": "Bob", "initiative": 10})
	readInitiative(t, gmClient)
	gmClient.send(t, "initiative.advance", map[string]any{"direction": "next"})
	readInitiative(t, gmClient)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	env := client.readEnvelope(t)
	if env.Type != "state.sync" {
		t.Fatalf("type = %q, want state.sync", env.Type)
	}

	var payload struct {
		Initiative initiativePayloadShape `json:"initiative"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal state.sync payload: %v", err)
	}
	if !namesEqual(payload.Initiative.names(), []string{"Bob"}) {
		t.Fatalf("entries = %v, want [Bob]", payload.Initiative.names())
	}
	if payload.Initiative.CurrentEntryID == nil {
		t.Fatalf("state.sync arrived with nobody up, mid-encounter")
	}
}
