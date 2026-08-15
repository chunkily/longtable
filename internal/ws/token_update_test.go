package ws

import (
	"encoding/json"
	"testing"
	"time"

	"longtable/internal/store"
)

// Editing a token is the first command whose broadcast depends on what
// the token used to be, not only on what it is now: crossing the hidden
// line in either direction has to tell Players something different from
// what it tells the GM. That asymmetry is what most of this covers.

func updatedToken(t *testing.T, env envelope) store.Token {
	t.Helper()

	var payload struct {
		ID           string  `json:"id"`
		Name         string  `json:"name"`
		ImageAssetID *string `json:"imageAssetId"`
		Width        float64 `json:"width"`
		Height       float64 `json:"height"`
		Visibility   string  `json:"visibility"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal %s payload: %v", env.Type, err)
	}
	return store.Token{
		ID:           payload.ID,
		Name:         payload.Name,
		ImageAssetID: payload.ImageAssetID,
		Width:        payload.Width,
		Height:       payload.Height,
		Visibility:   store.Visibility(payload.Visibility),
	}
}

func TestTokenUpdate_ChangesTheTokenAndTellsTheRoom(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync

	gmClient.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "name": "Hobgoblin", "width": 2, "height": 2, "visibility": "visible",
	})

	if env := gmClient.readEnvelope(t); env.Type != "token.updated" {
		t.Fatalf("gm event type = %q, want token.updated", env.Type)
	}
	env := playerClient.readEnvelope(t)
	if env.Type != "token.updated" {
		t.Fatalf("player event type = %q, want token.updated", env.Type)
	}
	got := updatedToken(t, env)
	if got.Name != "Hobgoblin" || got.Width != 2 || got.Height != 2 {
		t.Fatalf("broadcast token = %+v, want Hobgoblin at 2x2", got)
	}

	stored, err := r.ts.store.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if stored.Name != "Hobgoblin" || stored.Width != 2 {
		t.Fatalf("stored token = %+v, want the edit to have persisted", stored)
	}
}

// Position belongs to token.move. An edit dialog opened before a drag
// and submitted after one must not put the token back.
func TestTokenUpdate_LeavesThePositionAlone(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.move", map[string]any{"tokenId": token.ID, "x": 9, "y": 11})
	client.readEnvelope(t) // token.moved

	client.send(t, "token.update", map[string]any{"tokenId": token.ID, "name": "Hobgoblin"})
	client.readEnvelope(t) // token.updated

	stored, err := r.ts.store.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if stored.X != 9 || stored.Y != 11 {
		t.Fatalf("token at (%v, %v), want the move to have survived the edit", stored.X, stored.Y)
	}
}

// The owner is an editable field now, which means it follows the rule
// the rest of them follow: sent every time, and an edit that leaves it
// out is an edit that clears it. Taking a token back off a Player is a
// real edit, and the wire can't tell "left alone" from "unassign" — the
// same argument that decided it for a token's art.
//
// This test used to assert the opposite, back when nothing could set an
// owner and preserving it was the only way it survived at all.
func TestTokenUpdate_AssignsAndClearsTheOwner(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Bob's Fighter", store.VisibilityVisible)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "name": "Bob's Fighter", "ownerParticipantId": r.player.ID,
	})
	client.readEnvelope(t) // token.updated

	stored, err := r.ts.store.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if stored.OwnerParticipantID == nil || *stored.OwnerParticipantID != r.player.ID {
		t.Fatalf("owner = %v, want the player", stored.OwnerParticipantID)
	}

	client.send(t, "token.update", map[string]any{"tokenId": token.ID, "name": "Bob's Fighter"})
	client.readEnvelope(t) // token.updated

	stored, err = r.ts.store.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if stored.OwnerParticipantID != nil {
		t.Fatalf("owner = %v, want it cleared", *stored.OwnerParticipantID)
	}
}

// A participant ID is unguessable but it isn't scoped, so the same check
// that stands between a token and another room's art has to stand
// between it and another room's people.
func TestTokenUpdate_RefusesAnOwnerFromAnotherRoom(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	_, otherGM, err := r.ts.store.CreateRoom("Elsewhere", "Carol", "", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "name": "Goblin", "ownerParticipantId": otherGM.ID,
	})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("event type = %q, want error", env.Type)
	}

	stored, err := r.ts.store.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if stored.OwnerParticipantID != nil {
		t.Fatalf("owner = %v, want the edit refused outright", *stored.OwnerParticipantID)
	}
}

// Hiding a token a Player is looking at can't be expressed by an event
// that withholds itself from them, so they get a deletion — which is
// what has happened as far as their map is concerned.
func TestTokenUpdate_HidingTakesItOffThePlayersMap(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Ambusher", store.VisibilityVisible)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync

	gmClient.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "name": "Ambusher", "visibility": "hidden",
	})

	if env := gmClient.readEnvelope(t); env.Type != "token.updated" {
		t.Fatalf("gm event type = %q, want token.updated", env.Type)
	}
	env := playerClient.readEnvelope(t)
	if env.Type != "token.deleted" {
		t.Fatalf("player event type = %q, want token.deleted", env.Type)
	}
	if got := deletedTokenID(t, env); got != token.ID {
		t.Fatalf("deleted tokenId = %q, want %q", got, token.ID)
	}
	// And nothing else follows — in particular no token.updated leaking
	// the name of something they can no longer see.
	playerClient.expectNoMessage(t, 200*time.Millisecond)
}

// The mirror image: a Player who was never told the token existed needs
// the whole thing, not a diff against something they haven't got.
func TestTokenUpdate_RevealingSendsThePlayerTheWholeToken(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Ambusher", store.VisibilityHidden)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync

	gmClient.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "name": "Ambusher", "visibility": "visible",
	})

	if env := gmClient.readEnvelope(t); env.Type != "token.updated" {
		t.Fatalf("gm event type = %q, want token.updated", env.Type)
	}
	env := playerClient.readEnvelope(t)
	if env.Type != "token.updated" {
		t.Fatalf("player event type = %q, want token.updated", env.Type)
	}
	got := updatedToken(t, env)
	if got.ID != token.ID || got.Name != "Ambusher" {
		t.Fatalf("player got %+v, want the whole token so it can be added", got)
	}
}

// Editing a token that stays hidden must stay silent, or the sequence of
// events alone tells a Player something is there.
func TestTokenUpdate_EditingAStillHiddenTokenSaysNothingToPlayers(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Ambusher", store.VisibilityHidden)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync

	gmClient.send(t, "token.update", map[string]any{
		"tokenId": token.ID, "name": "Something Worse", "visibility": "hidden",
	})

	if env := gmClient.readEnvelope(t); env.Type != "token.updated" {
		t.Fatalf("gm event type = %q, want token.updated", env.Type)
	}
	playerClient.expectNoMessage(t, 200*time.Millisecond)
}

func TestTokenUpdate_PlayerMayNot(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.update", map[string]any{"tokenId": token.ID, "name": "Mine Now"})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}

	stored, err := r.ts.store.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if stored.Name != "Goblin" {
		t.Fatalf("name = %q, want it unchanged by a player", stored.Name)
	}
}

func TestTokenUpdate_TokenFromAnotherRoomFailsLikeAMissingOne(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	otherRoom, otherGM, err := r.ts.store.CreateRoom("Other", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	client := r.ts.connect(t, otherRoom.Slug, otherGM.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.update", map[string]any{"tokenId": token.ID, "name": "Theirs Now"})
	fromOtherRoom := errorMessage(t, client.readEnvelope(t))

	client.send(t, "token.update", map[string]any{
		"tokenId": "00000000-0000-4000-8000-000000000000", "name": "Nobody",
	})
	fromMissing := errorMessage(t, client.readEnvelope(t))

	if fromOtherRoom != fromMissing {
		t.Fatalf("errors differ: %q vs %q — the two must be indistinguishable", fromOtherRoom, fromMissing)
	}

	stored, err := r.ts.store.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if stored.Name != "Goblin" {
		t.Fatalf("name = %q, want it untouched by another room's GM", stored.Name)
	}
}

func TestTokenUpdate_RejectsMalformedPayloadAndBadVisibility(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	for _, payload := range []map[string]any{
		{},                    // no token, no name
		{"tokenId": token.ID}, // a token can't be left nameless
		{"tokenId": token.ID, "name": "Goblin", // and not every string is a visibility
			"visibility": "invisible"},
	} {
		client.send(t, "token.update", payload)
		if env := client.readEnvelope(t); env.Type != "error" {
			t.Fatalf("payload %v: type = %q, want error", payload, env.Type)
		}
	}
}
