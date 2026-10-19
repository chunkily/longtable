package ws

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"longtable/internal/store"
)

// The party is a roster of characters keyed across scenes, so most of
// what can go wrong is about copies: one too many, one on the wrong
// scene, or one left behind in the party after its character left.

type partyToken struct {
	ID                 string           `json:"id"`
	SceneID            string           `json:"sceneId"`
	Name               string           `json:"name"`
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

func readPartyToken(t *testing.T, c *testClient, wantType string) partyToken {
	t.Helper()

	env := c.readEnvelope(t)
	if env.Type != wantType {
		t.Fatalf("type = %q, want %s", env.Type, wantType)
	}
	var tok partyToken
	if err := json.Unmarshal(env.Payload, &tok); err != nil {
		t.Fatalf("unmarshal %s: %v", wantType, err)
	}
	return tok
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

// partyRoom is a tokenTestRoom with a second scene to stamp onto, and a
// GM and a Player both connected and past their state.sync.
type partyRoom struct {
	tokenTestRoom
	next   store.Scene
	gmc    *testClient
	pc     *testClient
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

// flag ticks a token into the party the way the edit dialog does: a full
// token.update carrying everything it already had, plus inParty.
func (r partyRoom) flag(t *testing.T, tok store.Token, inParty bool) partyToken {
	t.Helper()

	r.gmc.send(t, "token.update", map[string]any{
		"tokenId": tok.ID, "name": tok.Name, "imageAssetId": tok.ImageAssetID,
		"width": tok.Width, "height": tok.Height, "ownerParticipantId": tok.OwnerParticipantID,
		"visibility": string(tok.Visibility), "inParty": inParty,
	})
	return readPartyToken(t, r.gmc, "token.updated")
}

func (r partyRoom) partyToken(t *testing.T, name string, w float64, owner *string, visibility store.Visibility) store.Token {
	t.Helper()

	tok, err := r.ts.store.CreateToken(store.Token{
		SceneID: r.scene.ID, Name: name, X: 1, Y: 1, Width: w, Height: w,
		OwnerParticipantID: owner, Visibility: visibility,
		Trackers:   []store.Tracker{{Label: "HP", Value: new(int)}},
		Conditions: []string{"Poisoned"},
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	return tok
}

func TestPartyFlag_GivesTheTokenAKeyEveryoneIsToldAbout(t *testing.T) {
	r := newPartyRoom(t)
	aria := r.partyToken(t, "Aria", 1, &r.player.ID, store.VisibilityVisible)

	got := r.flag(t, aria, true)
	if got.PartyKey == nil || *got.PartyKey == "" {
		t.Fatalf("partyKey = %v, want a key", got.PartyKey)
	}
	if seen := readPartyToken(t, r.pc, "token.updated"); seen.PartyKey == nil || *seen.PartyKey != *got.PartyKey {
		t.Fatalf("player saw partyKey %v, want %q", seen.PartyKey, *got.PartyKey)
	}

	// Saving the dialog again with the box still ticked keeps the key it
	// has: a fresh one would make the token a different character from
	// the copies it already has on other scenes.
	again := r.flag(t, aria, true)
	if again.PartyKey == nil || *again.PartyKey != *got.PartyKey {
		t.Fatalf("partyKey changed on re-save: %v -> %v", *got.PartyKey, again.PartyKey)
	}
}

// What a copy is: the character, not the evening. Name, art, size, owner
// and visibility come with it; hit points, conditions and position don't.
func TestPartyAdd_CopiesTheCharacterButNotTheirState(t *testing.T) {
	r := newPartyRoom(t)
	ownerID := r.player.ID
	aria := r.partyToken(t, "Aria", 1, &ownerID, store.VisibilityVisible)
	brute := r.partyToken(t, "Brute", 2, nil, store.VisibilityVisible)
	ariaKey := *r.flag(t, aria, true).PartyKey
	r.pc.readEnvelope(t)
	bruteKey := *r.flag(t, brute, true).PartyKey
	r.pc.readEnvelope(t)

	r.gmc.send(t, "party.add", map[string]any{"sceneId": r.next.ID, "x": 5, "y": 5})

	byName := map[string]partyToken{}
	for range 2 {
		tok := readPartyToken(t, r.gmc, "token.created")
		byName[tok.Name] = tok
	}
	ids := readPartyAdded(t, r.gmc)

	copied := byName["Aria"]
	if copied.SceneID != r.next.ID || copied.ID == aria.ID {
		t.Fatalf("Aria's copy = %+v, want a new token on the boss room", copied)
	}
	if copied.OwnerParticipantID == nil || *copied.OwnerParticipantID != ownerID {
		t.Errorf("owner = %v, want %s", copied.OwnerParticipantID, ownerID)
	}
	if copied.PartyKey == nil || *copied.PartyKey != ariaKey {
		t.Errorf("partyKey = %v, want the original's %q — a copy is the same character", copied.PartyKey, ariaKey)
	}
	if v := copied.Trackers[0].Value; v != nil {
		t.Errorf("hit points came along as %d, want a blank slot", *v)
	}
	if len(copied.Conditions) != 0 {
		t.Errorf("conditions came along: %v", copied.Conditions)
	}
	if b := byName["Brute"]; b.Width != 2 || b.Height != 2 || *b.PartyKey != bruteKey {
		t.Errorf("Brute's copy = %+v, want 2x2 under its own key", b)
	}

	want := []string{byName["Aria"].ID, byName["Brute"].ID}
	slices.Sort(ids)
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Errorf("party.added = %v, want %v", ids, want)
	}

	// The Player hears about both, the same as any token made in the room.
	for range 2 {
		readPartyToken(t, r.pc, "token.created")
	}
}

// A Large companion beside a Medium character mustn't be put down
// overlapping it — the reason party.add spawns by footprint rather than
// as a batch of one size.
func TestPartyAdd_SpreadsMixedSizesWithoutOverlap(t *testing.T) {
	r := newPartyRoom(t)
	for _, w := range []float64{2, 1, 1} {
		tok := r.partyToken(t, "Member", w, nil, store.VisibilityVisible)
		r.flag(t, tok, true)
		r.pc.readEnvelope(t)
	}

	r.gmc.send(t, "party.add", map[string]any{"sceneId": r.next.ID, "x": 5, "y": 5})
	var placed []partyToken
	for range 3 {
		placed = append(placed, readPartyToken(t, r.gmc, "token.created"))
	}
	for i, a := range placed {
		for _, b := range placed[i+1:] {
			if overlaps(cell{a.X, a.Y}, a.Width, a.Height, cell{b.X, b.Y}, b.Width, b.Height) {
				t.Fatalf("%+v overlaps %+v", a, b)
			}
		}
	}
}

// Anyone already on the scene is skipped, which is what makes a second
// press harmless and lets a late arrival be added by pressing it again.
func TestPartyAdd_SkipsCharactersAlreadyOnTheScene(t *testing.T) {
	r := newPartyRoom(t)
	aria := r.partyToken(t, "Aria", 1, nil, store.VisibilityVisible)
	r.flag(t, aria, true)
	r.pc.readEnvelope(t)

	r.gmc.send(t, "party.add", map[string]any{"sceneId": r.next.ID, "x": 0, "y": 0})
	readPartyToken(t, r.gmc, "token.created")
	readPartyAdded(t, r.gmc)

	r.gmc.send(t, "party.add", map[string]any{"sceneId": r.next.ID, "x": 0, "y": 0})
	if msg := errorMessage(t, r.gmc.readEnvelope(t)); msg != "The whole party is already here." {
		t.Fatalf("second press: %q", msg)
	}

	// A character flagged since is the only one the next press brings.
	brom := r.partyToken(t, "Brom", 1, nil, store.VisibilityVisible)
	r.flag(t, brom, true)
	r.gmc.send(t, "party.add", map[string]any{"sceneId": r.next.ID, "x": 0, "y": 0})
	if tok := readPartyToken(t, r.gmc, "token.created"); tok.Name != "Brom" {
		t.Fatalf("added %q, want only Brom", tok.Name)
	}
	if ids := readPartyAdded(t, r.gmc); len(ids) != 1 {
		t.Fatalf("party.added = %v, want one id", ids)
	}
}

// The newest copy is what's copied, so art or a name changed on the boss
// room's Aria is what the scene after it gets.
func TestPartyAdd_CopiesTheNewestVersionOfEachCharacter(t *testing.T) {
	r := newPartyRoom(t)
	aria := r.partyToken(t, "Aria", 1, nil, store.VisibilityVisible)
	r.flag(t, aria, true)
	r.gmc.send(t, "party.add", map[string]any{"sceneId": r.next.ID, "x": 0, "y": 0})
	copied := readPartyToken(t, r.gmc, "token.created")
	readPartyAdded(t, r.gmc)

	renamed, err := r.ts.store.GetToken(copied.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	renamed.Name = "Aria the Bold"
	r.flag(t, renamed, true)

	third, err := r.ts.store.CreateScene(r.room.ID, "Epilogue", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	r.gmc.send(t, "party.add", map[string]any{"sceneId": third.ID, "x": 0, "y": 0})
	if tok := readPartyToken(t, r.gmc, "token.created"); tok.Name != "Aria the Bold" {
		t.Fatalf("copied %q, want the newest name", tok.Name)
	}
}

func TestPartyAdd_SaysSoWhenNobodyIsInTheParty(t *testing.T) {
	r := newPartyRoom(t)
	r.partyToken(t, "Goblin", 1, nil, store.VisibilityVisible)

	r.gmc.send(t, "party.add", map[string]any{"sceneId": r.next.ID, "x": 0, "y": 0})
	if msg := errorMessage(t, r.gmc.readEnvelope(t)); msg != "No tokens are in the party yet." {
		t.Fatalf("got %q", msg)
	}
}

// Both halves are a GM's: a Player can't add the party, and an inParty a
// Player's client sends on a token they own is ignored like the other
// GM fields on token.update.
func TestParty_IsTheGMs(t *testing.T) {
	r := newPartyRoom(t)
	ownerID := r.player.ID
	mine := r.partyToken(t, "Bob's Fighter", 1, &ownerID, store.VisibilityVisible)

	r.pc.send(t, "party.add", map[string]any{"sceneId": r.next.ID, "x": 0, "y": 0})
	if msg := errorMessage(t, r.pc.readEnvelope(t)); msg != "that action is GM-only" {
		t.Fatalf("party.add from a Player: %q", msg)
	}

	r.pc.send(t, "token.update", map[string]any{
		"tokenId": mine.ID, "inParty": true,
		"trackers": []map[string]any{{"label": "HP", "value": 3}},
	})
	if got := readPartyToken(t, r.pc, "token.updated"); got.PartyKey != nil {
		t.Fatalf("a Player put their token in the party: key %q", *got.PartyKey)
	}
}

// Unticking is about the character, so every copy on every scene leaves
// with it and the room is told about each — including copies on a scene
// nobody here is looking at, which their clients drop on arrival. A hidden
// copy is still withheld from the Player.
func TestPartyLeave_TakesEveryCopyOutAndTellsTheRoom(t *testing.T) {
	r := newPartyRoom(t)
	aria := r.partyToken(t, "Aria", 1, nil, store.VisibilityVisible)
	key := *r.flag(t, aria, true).PartyKey
	r.pc.readEnvelope(t)

	elsewhere, err := r.ts.store.CreateToken(store.Token{
		SceneID: r.next.ID, Name: "Aria", Width: 1, Height: 1, PartyKey: key,
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	hiddenCopy, err := r.ts.store.CreateToken(store.Token{
		SceneID: r.next.ID, Name: "Aria", Width: 1, Height: 1, PartyKey: key, Visibility: store.VisibilityHidden,
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	r.flag(t, aria, false) // the GM's own echo of the edit itself
	told := map[string]bool{}
	for range 2 {
		tok := readPartyToken(t, r.gmc, "token.updated")
		if tok.PartyKey != nil {
			t.Errorf("%s still keyed", tok.ID)
		}
		told[tok.ID] = true
	}
	if !told[elsewhere.ID] || !told[hiddenCopy.ID] {
		t.Fatalf("GM told about %v, want both other copies", told)
	}

	for _, id := range []string{aria.ID, elsewhere.ID, hiddenCopy.ID} {
		got, err := r.ts.store.GetToken(id)
		if err != nil {
			t.Fatalf("GetToken: %v", err)
		}
		if got.PartyKey != "" {
			t.Errorf("%s still has party key %q", id, got.PartyKey)
		}
	}

	// The Player: the edited token, then the visible copy — and nothing
	// about the hidden one.
	gotIDs := []string{}
	for range 2 {
		gotIDs = append(gotIDs, readPartyToken(t, r.pc, "token.updated").ID)
	}
	if slices.Contains(gotIDs, hiddenCopy.ID) {
		t.Fatalf("the Player was told about a hidden copy")
	}
	r.pc.expectNoMessage(t, 200*time.Millisecond)
}

// Undoing the deletion of a party character puts it back in the party,
// under the key its other copies still carry — which only a GM can do.
func TestTokenCreate_GMCanRestoreAPartyKey(t *testing.T) {
	r := newPartyRoom(t)
	key := "0b6f5c4e-6a4f-4a8e-9c1d-2f3e4a5b6c7d"

	r.gmc.send(t, "token.create", map[string]any{"sceneId": r.scene.ID, "name": "Aria", "partyKey": key})
	if got := readPartyToken(t, r.gmc, "token.created"); got.PartyKey == nil || *got.PartyKey != key {
		t.Fatalf("GM restore: partyKey %v, want %q", got.PartyKey, key)
	}
	r.pc.readEnvelope(t)

	r.pc.send(t, "token.create", map[string]any{"sceneId": r.scene.ID, "name": "Mine", "partyKey": key})
	if got := readPartyToken(t, r.pc, "token.created"); got.PartyKey != nil {
		t.Fatalf("a Player joined the party through token.create: %q", *got.PartyKey)
	}

	r.gmc.readEnvelope(t) // the Player's token.created
	r.gmc.send(t, "token.create", map[string]any{"sceneId": r.scene.ID, "name": "Bad", "partyKey": "{" + key + "}"})
	if msg := errorMessage(t, r.gmc.readEnvelope(t)); msg != "partyKey must be a canonical UUID" {
		t.Fatalf("non-canonical key: %q", msg)
	}
}
