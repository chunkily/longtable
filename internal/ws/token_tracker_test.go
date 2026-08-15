package ws

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"longtable/internal/store"
)

// Trackers and conditions are the first fields on a token that someone
// other than the GM may change, so most of what follows is about the
// role check having become per-field rather than one gate at the top.

// ownedToken is newTokenTestRoom's token() with an owner, which is the
// precondition for every Player-side case here.
func (r tokenTestRoom) ownedToken(t *testing.T, name string, visibility store.Visibility, ownerID string) store.Token {
	t.Helper()

	token, err := r.ts.store.CreateToken(store.Token{
		SceneID: r.scene.ID, Name: name, X: 3, Y: 4, Width: 1, Height: 1,
		Visibility: visibility, OwnerParticipantID: &ownerID,
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	return token
}

type trackerPayload struct {
	Label string `json:"label"`
	Value *int   `json:"value"`
}

func broadcastTrackers(t *testing.T, env envelope) ([]trackerPayload, []string) {
	t.Helper()

	var payload struct {
		Trackers   []trackerPayload `json:"trackers"`
		Conditions []string         `json:"conditions"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal %s payload: %v", env.Type, err)
	}
	return payload.Trackers, payload.Conditions
}

func TestTokenUpdate_GMSetsTrackersAndConditionsForTheWholeRoom(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync

	gmClient.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "name": "Goblin", "visibility": "visible",
		"trackers": []map[string]any{
			{"label": "HP", "value": 7},
			{"label": "AC", "value": 15},
			{"label": "", "value": nil},
		},
		"conditions": []string{"Prone"},
	})
	gmClient.readEnvelope(t) // token.updated

	trackers, conditions := broadcastTrackers(t, playerClient.readEnvelope(t))
	if len(trackers) != store.TrackerSlots {
		t.Fatalf("broadcast carried %d trackers, want %d", len(trackers), store.TrackerSlots)
	}
	if trackers[0].Label != "HP" || trackers[0].Value == nil || *trackers[0].Value != 7 {
		t.Fatalf("first tracker = %+v, want HP 7", trackers[0])
	}
	if len(conditions) != 1 || conditions[0] != "Prone" {
		t.Fatalf("conditions = %v, want [Prone]", conditions)
	}

	stored, err := r.ts.store.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if stored.Trackers[1].Label != "AC" || stored.Trackers[1].Value == nil || *stored.Trackers[1].Value != 15 {
		t.Fatalf("stored second tracker = %+v, want AC 15", stored.Trackers[1])
	}
}

// The whole point of the per-field split: a Player tracks their own
// damage without having to ask the GM, and without becoming one.
func TestTokenUpdate_OwnerMaySetTrackersWithoutBeingGM(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.ownedToken(t, "Bob's Fighter", store.VisibilityVisible, r.player.ID)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync

	playerClient.send(t, "token.update", map[string]any{
		"tokenId":    token.ID,
		"trackers":   []map[string]any{{"label": "HP", "value": 3}},
		"conditions": []string{"Concentrating"},
	})

	if env := playerClient.readEnvelope(t); env.Type != "token.updated" {
		t.Fatalf("player event type = %q, want token.updated", env.Type)
	}
	// And it reaches the rest of the table, not just the person who typed it.
	env := gmClient.readEnvelope(t)
	if env.Type != "token.updated" {
		t.Fatalf("gm event type = %q, want token.updated", env.Type)
	}
	trackers, conditions := broadcastTrackers(t, env)
	if trackers[0].Value == nil || *trackers[0].Value != 3 {
		t.Fatalf("gm was told %+v, want HP 3", trackers[0])
	}
	if len(conditions) != 1 || conditions[0] != "Concentrating" {
		t.Fatalf("gm was told conditions %v, want [Concentrating]", conditions)
	}
}

// A Player's update is applied to the trackers and conditions only. The
// GM-only fields aren't rejected, they're ignored — the loaded token
// keeps them, which is the same in-place edit that protects a field the
// command doesn't carry at all.
func TestTokenUpdate_OwnerCannotRenameRetagOrRevealTheirOwnToken(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.ownedToken(t, "Bob's Fighter", store.VisibilityVisible, r.player.ID)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "name": "Bob's Dragon", "visibility": "hidden",
		"width": 4, "height": 4, "ownerParticipantId": nil,
		"trackers": []map[string]any{{"label": "HP", "value": 12}},
	})
	if env := client.readEnvelope(t); env.Type != "token.updated" {
		t.Fatalf("event type = %q, want token.updated", env.Type)
	}

	stored, err := r.ts.store.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if stored.Name != "Bob's Fighter" {
		t.Fatalf("name = %q, want a Player's rename ignored", stored.Name)
	}
	if stored.Visibility != store.VisibilityVisible {
		t.Fatalf("visibility = %q, want a Player unable to hide a token", stored.Visibility)
	}
	if stored.Width != 1 {
		t.Fatalf("width = %v, want a Player unable to resize", stored.Width)
	}
	if stored.OwnerParticipantID == nil || *stored.OwnerParticipantID != r.player.ID {
		t.Fatalf("owner = %v, want a Player unable to unassign themselves", stored.OwnerParticipantID)
	}
	if stored.Trackers[0].Value == nil || *stored.Trackers[0].Value != 12 {
		t.Fatalf("tracker = %+v, want the one field they may set applied", stored.Trackers[0])
	}
}

func TestTokenUpdate_PlayerCannotSetTrackersOnSomeoneElsesToken(t *testing.T) {
	r := newTokenTestRoom(t)
	// Owned by the GM, so it is a token that exists and is visible and is
	// simply not this Player's.
	token := r.ownedToken(t, "The GM's Wolf", store.VisibilityVisible, r.gm.ID)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "trackers": []map[string]any{{"label": "HP", "value": 0}},
	})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("event type = %q, want error", env.Type)
	}

	stored, err := r.ts.store.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if stored.Trackers[0].Value != nil {
		t.Fatalf("tracker = %+v, want no effect at all", stored.Trackers[0])
	}
}

// An unowned token — which is most of them — is nobody's to edit but the
// GM's. This is the case the old single gate covered, kept honest now
// that the gate has moved.
func TestTokenUpdate_PlayerCannotSetTrackersOnAnUnownedToken(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "trackers": []map[string]any{{"label": "HP", "value": 1}},
	})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("event type = %q, want error", env.Type)
	}
}

// A GM can own a hidden token *on a Player's behalf* — prepping an
// ambush with the Player's own character, say. The Player still can't
// see it, so the refusal must not be the thing that tells them it's
// there: it has to be word-for-word what a token that doesn't exist
// says.
func TestTokenUpdate_HiddenTokenIsRefusedToItsOwnerLikeAMissingOne(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.ownedToken(t, "Doppelganger", store.VisibilityHidden, r.player.ID)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "trackers": []map[string]any{{"label": "HP", "value": 1}},
	})
	fromHidden := errorMessage(t, client.readEnvelope(t))

	client.send(t, "token.update", map[string]any{
		"tokenId":  "00000000-0000-4000-8000-000000000000",
		"trackers": []map[string]any{{"label": "HP", "value": 1}},
	})
	fromMissing := errorMessage(t, client.readEnvelope(t))

	if fromHidden != fromMissing {
		t.Fatalf("errors differ: %q vs %q — the two must be indistinguishable", fromHidden, fromMissing)
	}
}

// A Player's edit must not trip the visible -> hidden branch, which
// sends a token.deleted. The permission check and the broadcast filter
// are two separate decisions about roles in the same handler now, and
// this pins that the second one still reads the token rather than the
// sender.
func TestTokenUpdate_OwnersEditSendsNothingBesidesTheUpdate(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.ownedToken(t, "Bob's Fighter", store.VisibilityVisible, r.player.ID)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync

	playerClient.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "trackers": []map[string]any{{"label": "HP", "value": 5}},
	})
	playerClient.readEnvelope(t) // its own echo
	if env := gmClient.readEnvelope(t); env.Type != "token.updated" {
		t.Fatalf("gm event type = %q, want token.updated", env.Type)
	}
	// Only the GM is asked. expectNoMessage ends by letting a read time
	// out, which closes that client's socket — so the hub's disconnect
	// cleanup broadcasts a measure.ended to everyone else in the room, and
	// a second client asked the same question afterwards would read that
	// instead of nothing. Every other use of this helper is the last line
	// of its test for the same reason.
	gmClient.expectNoMessage(t, 200*time.Millisecond)
}

// Zero is a value and null is an empty slot. A creature on 0 hit points
// is the single most important number a tracker ever holds, so the two
// must not collapse into each other anywhere on the round trip.
func TestTokenUpdate_ZeroIsAValueNotAnEmptySlot(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "name": "Goblin",
		"trackers": []map[string]any{
			{"label": "HP", "value": 0},
			{"label": "AC", "value": nil},
		},
	})
	trackers, _ := broadcastTrackers(t, client.readEnvelope(t))

	if trackers[0].Value == nil || *trackers[0].Value != 0 {
		t.Fatalf("HP = %+v, want a set value of 0", trackers[0])
	}
	if trackers[1].Value != nil {
		t.Fatalf("AC = %+v, want an empty slot", trackers[1])
	}

	stored, err := r.ts.store.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if stored.Trackers[0].Value == nil || *stored.Trackers[0].Value != 0 {
		t.Fatalf("stored HP = %+v, want 0 to have survived the database", stored.Trackers[0])
	}
	if stored.Trackers[1].Value != nil {
		t.Fatalf("stored AC = %+v, want the empty slot to have stayed empty", stored.Trackers[1])
	}
}

// The same "every field every time" rule the rest of token.update
// follows: an edit that omits the trackers is an edit that clears them.
// Recorded as a test because it's the rule most likely to be broken by a
// form that predates the fields.
func TestTokenUpdate_OmittingTrackersClearsThem(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "name": "Goblin",
		"trackers":   []map[string]any{{"label": "HP", "value": 9}},
		"conditions": []string{"Prone"},
	})
	client.readEnvelope(t) // token.updated

	client.send(t, "token.update", map[string]any{"tokenId": token.ID, "name": "Goblin"})
	client.readEnvelope(t) // token.updated

	stored, err := r.ts.store.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if stored.Trackers[0].Value != nil || stored.Trackers[0].Label != "" {
		t.Fatalf("tracker = %+v, want it cleared by an update that didn't carry it", stored.Trackers[0])
	}
	if len(stored.Conditions) != 0 {
		t.Fatalf("conditions = %v, want them cleared too", stored.Conditions)
	}
}

func TestTokenUpdate_NormalisesConditions(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "name": "Goblin",
		"conditions": []string{" Prone ", "prone", "", "Poisoned"},
	})
	_, conditions := broadcastTrackers(t, client.readEnvelope(t))

	// Trimmed, blanks dropped, and "prone" folded into the "Prone" already
	// there under the spelling that arrived first.
	want := []string{"Prone", "Poisoned"}
	if len(conditions) != len(want) {
		t.Fatalf("conditions = %v, want %v", conditions, want)
	}
	for i := range want {
		if conditions[i] != want[i] {
			t.Fatalf("conditions = %v, want %v", conditions, want)
		}
	}
}

func TestTokenUpdate_RefusesOversizedTrackersAndConditions(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	tooMany := make([]string, maxTokenConditions+1)
	for i := range tooMany {
		tooMany[i] = string(rune('a' + i))
	}

	for name, payload := range map[string]map[string]any{
		"a fourth tracker slot": {"tokenId": token.ID, "name": "Goblin", "trackers": []map[string]any{
			{"label": "a"}, {"label": "b"}, {"label": "c"}, {"label": "d"},
		}},
		"an overlong label": {"tokenId": token.ID, "name": "Goblin", "trackers": []map[string]any{
			{"label": strings.Repeat("x", maxTrackerLabel+1), "value": 1},
		}},
		"an overlong condition": {"tokenId": token.ID, "name": "Goblin",
			"conditions": []string{strings.Repeat("x", maxConditionText+1)}},
		"too many conditions": {"tokenId": token.ID, "name": "Goblin", "conditions": tooMany},
	} {
		client.send(t, "token.update", payload)
		if env := client.readEnvelope(t); env.Type != "error" {
			t.Fatalf("%s: type = %q, want error", name, env.Type)
		}
	}

	stored, err := r.ts.store.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if stored.Trackers[0].Label != "" {
		t.Fatalf("tracker = %+v, want every refusal to have left the token alone", stored.Trackers[0])
	}
}

// Undoing a deletion rebuilds the token from the client's payload alone,
// so token.create has to carry the trackers too — otherwise a token
// comes back from an undo on full health.
func TestTokenCreate_RestoresTrackersUnderTheOriginalID(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	id := "11111111-1111-4111-8111-111111111111"
	client.send(t, "token.create", map[string]any{
		"tokenIds": []string{id}, "sceneId": r.scene.ID, "name": "Goblin", "x": 1, "y": 1,
		"trackers":   []map[string]any{{"label": "HP", "value": 4}},
		"conditions": []string{"Prone"},
	})
	if env := client.readEnvelope(t); env.Type != "token.created" {
		t.Fatalf("event type = %q, want token.created", env.Type)
	}

	stored, err := r.ts.store.GetToken(id)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if stored.Trackers[0].Label != "HP" || stored.Trackers[0].Value == nil || *stored.Trackers[0].Value != 4 {
		t.Fatalf("tracker = %+v, want the restored HP 4", stored.Trackers[0])
	}
	if len(stored.Conditions) != 1 || stored.Conditions[0] != "Prone" {
		t.Fatalf("conditions = %v, want [Prone]", stored.Conditions)
	}
}
