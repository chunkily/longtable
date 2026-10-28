package ws

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"longtable/internal/store"
)

// The party is defined by the GM, member by member, and party.add puts
// down a copy of each one not already on the scene. What's pinned here is
// the boundary (GM-only, and the roster is never sent to a Player), the
// checks a member shares with a token, and what a copy is.

type partyToken struct {
	ID                 string           `json:"id"`
	SceneID            string           `json:"sceneId"`
	Name               string           `json:"name"`
	ImageAssetID       *string          `json:"imageAssetId"`
	X                  float64          `json:"x"`
	Y                  float64          `json:"y"`
	Width              float64          `json:"width"`
	Height             float64          `json:"height"`
	OwnerParticipantID *string          `json:"ownerParticipantId"`
	Visibility         string           `json:"visibility"`
	PartyKey           *string          `json:"partyKey"`
	Trackers           []trackerPayload `json:"trackers"`
	Conditions         []string         `json:"conditions"`
}

type partyMemberShape struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	ImageAssetID       *string `json:"imageAssetId"`
	Width              float64 `json:"width"`
	OwnerParticipantID *string `json:"ownerParticipantId"`
	Attending          bool    `json:"attending"`
}

func readPartyToken(t *testing.T, c *testClient) partyToken {
	t.Helper()

	env := c.readEnvelope(t)
	if env.Type != "token.created" {
		t.Fatalf("type = %q, want token.created", env.Type)
	}
	var tok partyToken
	if err := json.Unmarshal(env.Payload, &tok); err != nil {
		t.Fatalf("unmarshal token.created: %v", err)
	}
	return tok
}

func readParty(t *testing.T, c *testClient) []partyMemberShape {
	t.Helper()

	env := c.readEnvelope(t)
	if env.Type != "party.updated" {
		t.Fatalf("type = %q, want party.updated", env.Type)
	}
	var payload struct {
		Members []partyMemberShape `json:"members"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal party.updated: %v", err)
	}
	return payload.Members
}

func readPartyAdded(t *testing.T, c *testClient) []string {
	t.Helper()

	env := c.readEnvelope(t)
	if env.Type != "party.added" {
		t.Fatalf("type = %q, want party.added", env.Type)
	}
	var payload struct {
		TokenIDs []string `json:"tokenIds"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal party.added: %v", err)
	}
	return payload.TokenIDs
}

// partyRoom is a tokenTestRoom with a second scene to put the party on,
// and a GM and a Player both connected and past their state.sync.
type partyRoom struct {
	tokenTestRoom
	next store.Scene
	gmc  *testClient
	pc   *testClient
}

func newPartyRoom(t *testing.T) partyRoom {
	t.Helper()

	r := newTokenTestRoom(t)
	next, err := r.ts.store.CreateScene(r.room.ID, "Boss room", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	gmc := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmc.readEnvelope(t) // state.sync
	pc := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	pc.readEnvelope(t) // state.sync
	return partyRoom{tokenTestRoom: r, next: next, gmc: gmc, pc: pc}
}

// member adds a party member as the GM's dialog would, and returns the
// roster that comes back.
func (r partyRoom) member(t *testing.T, fields map[string]any) []partyMemberShape {
	t.Helper()

	r.gmc.send(t, "party.memberCreate", fields)
	return readParty(t, r.gmc)
}

func (r partyRoom) add(t *testing.T, sceneID string) {
	t.Helper()
	r.gmc.send(t, "party.add", map[string]any{"sceneId": sceneID, "x": 5, "y": 5})
}

// The roster goes to GMs only — on every change and in state.sync — so a
// Player's client never holds it. The copies, once down, are everyone's.
func TestPartyMember_TheRosterIsTheGMsAlone(t *testing.T) {
	r := newPartyRoom(t)

	members := r.member(t, map[string]any{"name": "Aria", "ownerParticipantId": r.player.ID})
	if len(members) != 1 || members[0].Name != "Aria" || *members[0].OwnerParticipantID != r.player.ID {
		t.Fatalf("roster = %+v", members)
	}

	gmAgain := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	var gmSync struct {
		Party []partyMemberShape `json:"party"`
	}
	if err := json.Unmarshal(gmAgain.readEnvelope(t).Payload, &gmSync); err != nil {
		t.Fatalf("unmarshal GM state.sync: %v", err)
	}
	if len(gmSync.Party) != 1 {
		t.Fatalf("GM's state.sync party = %+v, want Aria", gmSync.Party)
	}

	playerAgain := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	var playerSync map[string]json.RawMessage
	if err := json.Unmarshal(playerAgain.readEnvelope(t).Payload, &playerSync); err != nil {
		t.Fatalf("unmarshal Player state.sync: %v", err)
	}
	if _, ok := playerSync["party"]; ok {
		t.Fatal("a Player's state.sync carried the party")
	}

	r.pc.expectNoMessage(t, 200*time.Millisecond)
}

func TestPartyMember_PlayersCantTouchIt(t *testing.T) {
	r := newPartyRoom(t)
	id := r.member(t, map[string]any{"name": "Aria"})[0].ID

	for _, cmd := range []struct {
		typ     string
		payload map[string]any
	}{
		{"party.memberCreate", map[string]any{"name": "Mine"}},
		{"party.memberUpdate", map[string]any{"memberId": id, "name": "Mine"}},
		{"party.memberDelete", map[string]any{"memberId": id}},
		{"party.add", map[string]any{"sceneId": r.next.ID}},
	} {
		r.pc.send(t, cmd.typ, cmd.payload)
		if msg := errorMessage(t, r.pc.readEnvelope(t)); msg != "that action is GM-only" {
			t.Fatalf("%s from a Player: %q", cmd.typ, msg)
		}
	}
}

// A member is what a token will be made from, so it gets a GM token's
// checks: art from this room's library, an owner sitting in this room.
func TestPartyMember_IsCheckedLikeAToken(t *testing.T) {
	r := newPartyRoom(t)
	other, _, err := r.ts.store.CreateRoom("Other", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	stranger, err := r.ts.store.JoinRoom(other.ID, "Stranger", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	for _, c := range []struct {
		payload map[string]any
		want    string
	}{
		{map[string]any{"name": "   "}, "A party member needs a name."},
		{map[string]any{"name": "Aria", "imageAssetId": "not-in-this-room"}, "asset not found"},
		{map[string]any{"name": "Aria", "ownerParticipantId": stranger.ID}, "owner not found"},
	} {
		r.gmc.send(t, "party.memberCreate", c.payload)
		if msg := errorMessage(t, r.gmc.readEnvelope(t)); msg != c.want {
			t.Errorf("%v: got %q, want %q", c.payload, msg, c.want)
		}
	}
}

func TestPartyMember_HasACap(t *testing.T) {
	r := newPartyRoom(t)
	for i := range maxPartySize {
		r.member(t, map[string]any{"name": fmt.Sprintf("Member %d", i+1)})
	}
	r.gmc.send(t, "party.memberCreate", map[string]any{"name": "One too many"})
	want := fmt.Sprintf("A party can have up to %d members.", maxPartySize)
	if msg := errorMessage(t, r.gmc.readEnvelope(t)); msg != want {
		t.Fatalf("got %q, want %q", msg, want)
	}
}

func TestPartyMember_EditAndRemove(t *testing.T) {
	r := newPartyRoom(t)
	id := r.member(t, map[string]any{"name": "Aria"})[0].ID
	r.member(t, map[string]any{"name": "Brom"})

	r.gmc.send(t, "party.memberUpdate", map[string]any{"memberId": id, "name": "Aria the Bold", "width": 2, "height": 2})
	members := readParty(t, r.gmc)
	if members[0].Name != "Aria the Bold" || members[0].Width != 2 {
		t.Fatalf("after edit: %+v", members)
	}

	r.gmc.send(t, "party.memberDelete", map[string]any{"memberId": id})
	if members := readParty(t, r.gmc); len(members) != 1 || members[0].Name != "Brom" {
		t.Fatalf("after delete: %+v", members)
	}

	r.gmc.send(t, "party.memberDelete", map[string]any{"memberId": id})
	if msg := errorMessage(t, r.gmc.readEnvelope(t)); msg != "party member not found" {
		t.Fatalf("deleting it twice: %q", msg)
	}
}

// What a copy is: the member, and not the evening. Name, art, size and
// owner; visible, blank trackers, no conditions; the member's id as its
// key. Spread by footprint, so the Large companion doesn't land on anyone.
func TestPartyAdd_PutsDownACopyOfEachMember(t *testing.T) {
	r := newPartyRoom(t)
	r.member(t, map[string]any{"name": "Aria", "ownerParticipantId": r.player.ID})
	r.member(t, map[string]any{"name": "Wolf", "width": 2, "height": 2})
	members := r.member(t, map[string]any{"name": "Brom"})

	r.add(t, r.next.ID)
	var placed []partyToken
	for range 3 {
		placed = append(placed, readPartyToken(t, r.gmc))
	}
	ids := readPartyAdded(t, r.gmc)

	if got := []string{placed[0].Name, placed[1].Name, placed[2].Name}; !slices.Equal(got, []string{"Aria", "Wolf", "Brom"}) {
		t.Fatalf("put down %v, want the party in the order it was added", got)
	}
	aria := placed[0]
	if aria.SceneID != r.next.ID || aria.Visibility != "visible" {
		t.Errorf("Aria's copy = %+v", aria)
	}
	if aria.OwnerParticipantID == nil || *aria.OwnerParticipantID != r.player.ID {
		t.Errorf("owner = %v, want %s", aria.OwnerParticipantID, r.player.ID)
	}
	if aria.PartyKey == nil || *aria.PartyKey != members[0].ID {
		t.Errorf("partyKey = %v, want the member's id %s", aria.PartyKey, members[0].ID)
	}
	if aria.Trackers[0].Value != nil || len(aria.Conditions) != 0 {
		t.Errorf("a copy came with state: %+v", aria)
	}
	for i, a := range placed {
		for _, b := range placed[i+1:] {
			if overlaps(cell{a.X, a.Y}, a.Width, a.Height, cell{b.X, b.Y}, b.Width, b.Height) {
				t.Fatalf("%s overlaps %s", a.Name, b.Name)
			}
		}
	}

	want := []string{placed[0].ID, placed[1].ID, placed[2].ID}
	if !slices.Equal(ids, want) {
		t.Errorf("party.added = %v, want %v", ids, want)
	}

	// The Player hears about the tokens like any others — just never the
	// roster they came from.
	for range 3 {
		readPartyToken(t, r.pc)
	}
}

// Anyone already on the scene is skipped, which makes a second press
// harmless and lets a member added since be put down on its own.
func TestPartyAdd_SkipsMembersAlreadyOnTheScene(t *testing.T) {
	r := newPartyRoom(t)
	r.member(t, map[string]any{"name": "Aria"})

	r.add(t, r.next.ID)
	readPartyToken(t, r.gmc)
	readPartyAdded(t, r.gmc)

	r.add(t, r.next.ID)
	if msg := errorMessage(t, r.gmc.readEnvelope(t)); msg != "The whole party is already here." {
		t.Fatalf("second press: %q", msg)
	}

	r.member(t, map[string]any{"name": "Brom"})
	r.add(t, r.next.ID)
	if tok := readPartyToken(t, r.gmc); tok.Name != "Brom" {
		t.Fatalf("put down %q, want only Brom", tok.Name)
	}
	if ids := readPartyAdded(t, r.gmc); len(ids) != 1 {
		t.Fatalf("party.added = %v, want one id", ids)
	}
}

// Editing a member changes the next copy, not the ones already down.
func TestPartyAdd_CopiesTheMemberAsItIsNow(t *testing.T) {
	r := newPartyRoom(t)
	id := r.member(t, map[string]any{"name": "Aria"})[0].ID
	r.add(t, r.scene.ID)
	first := readPartyToken(t, r.gmc)
	readPartyAdded(t, r.gmc)

	r.gmc.send(t, "party.memberUpdate", map[string]any{"memberId": id, "name": "Aria the Bold"})
	readParty(t, r.gmc)

	r.add(t, r.next.ID)
	if tok := readPartyToken(t, r.gmc); tok.Name != "Aria the Bold" {
		t.Fatalf("copied %q, want the member as edited", tok.Name)
	}
	if kept, err := r.ts.store.GetToken(first.ID); err != nil || kept.Name != "Aria" {
		t.Fatalf("the copy already down changed: %+v, %v", kept, err)
	}
}

func TestPartyAdd_SaysSoWhenThereIsNoParty(t *testing.T) {
	r := newPartyRoom(t)
	r.add(t, r.next.ID)
	if msg := errorMessage(t, r.gmc.readEnvelope(t)); msg != "No one is in the party yet." {
		t.Fatalf("got %q", msg)
	}
}

// Undoing the deletion of a party copy puts it back under the member it
// came from, so the next Add party still skips it — which only a GM can
// do.
func TestTokenCreate_GMCanRestoreAPartyKey(t *testing.T) {
	r := newPartyRoom(t)
	key := "0b6f5c4e-6a4f-4a8e-9c1d-2f3e4a5b6c7d"

	r.gmc.send(t, "token.create", map[string]any{"sceneId": r.scene.ID, "name": "Aria", "partyKey": key})
	if got := readPartyToken(t, r.gmc); got.PartyKey == nil || *got.PartyKey != key {
		t.Fatalf("GM restore: partyKey %v, want %q", got.PartyKey, key)
	}
	r.pc.readEnvelope(t)

	r.pc.send(t, "token.create", map[string]any{"sceneId": r.scene.ID, "name": "Mine", "partyKey": key})
	if got := readPartyToken(t, r.pc); got.PartyKey != nil {
		t.Fatalf("a Player's token.create set a party key: %q", *got.PartyKey)
	}

	r.gmc.readEnvelope(t) // the Player's token.created
	r.gmc.send(t, "token.create", map[string]any{"sceneId": r.scene.ID, "name": "Bad", "partyKey": "{" + key + "}"})
	if msg := errorMessage(t, r.gmc.readEnvelope(t)); msg != "partyKey must be a canonical UUID" {
		t.Fatalf("non-canonical key: %q", msg)
	}
}

// A new member is attending, so a GM who never touches the checkbox gets
// the behaviour from before it existed.
func TestPartyMember_StartsAttending(t *testing.T) {
	r := newPartyRoom(t)
	if m := r.member(t, map[string]any{"name": "Aria"})[0]; !m.Attending {
		t.Fatalf("new member = %+v, want attending", m)
	}
}

// Only who's at the table tonight is put down — and someone unticked who
// is later ticked is put down on the next press, like anyone missing.
func TestPartyAdd_SkipsMembersWhoAreNotAttending(t *testing.T) {
	r := newPartyRoom(t)
	r.member(t, map[string]any{"name": "Aria"})
	brom := r.member(t, map[string]any{"name": "Brom", "attending": false})[1]
	if brom.Attending {
		t.Fatalf("Brom = %+v, want not attending", brom)
	}

	r.add(t, r.next.ID)
	if tok := readPartyToken(t, r.gmc); tok.Name != "Aria" {
		t.Fatalf("put down %q, want only Aria", tok.Name)
	}
	if ids := readPartyAdded(t, r.gmc); len(ids) != 1 {
		t.Fatalf("party.added = %v, want one id", ids)
	}

	r.gmc.send(t, "party.memberUpdate", map[string]any{"memberId": brom.ID, "name": "Brom", "attending": true})
	readParty(t, r.gmc)
	r.add(t, r.next.ID)
	if tok := readPartyToken(t, r.gmc); tok.Name != "Brom" {
		t.Fatalf("put down %q, want Brom now he's here", tok.Name)
	}
}

// Nobody ticked is its own answer: "the whole party is already here"
// would be untrue of a scene with nobody on it.
func TestPartyAdd_SaysSoWhenNobodyIsAttending(t *testing.T) {
	r := newPartyRoom(t)
	r.member(t, map[string]any{"name": "Aria", "attending": false})
	r.add(t, r.next.ID)
	if msg := errorMessage(t, r.gmc.readEnvelope(t)); msg != "No one in the party is ticked as attending." {
		t.Fatalf("got %q", msg)
	}
}
