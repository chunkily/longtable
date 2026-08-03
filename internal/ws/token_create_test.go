package ws

import (
	"encoding/json"
	"testing"
)

// What a token carries at birth beyond a name and a position: how many
// squares it stands on, and whose it is. Both are optional and both have
// a defined answer when left out, which is most of what's here — plus
// the room boundary around the owner, since a participant ID is
// unguessable but not scoped.

func TestTokenCreate_RecordsSizeAndOwner(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.create", map[string]any{
		"sceneId": r.scene.ID, "name": "Bob's Fighter",
		"width": 2, "height": 2, "ownerParticipantId": r.player.ID,
	})

	env := client.readEnvelope(t)
	if env.Type != "token.created" {
		t.Fatalf("type = %q, want token.created", env.Type)
	}
	var payload struct {
		ID                 string  `json:"id"`
		Width              float64 `json:"width"`
		Height             float64 `json:"height"`
		OwnerParticipantID *string `json:"ownerParticipantId"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal token.created payload: %v", err)
	}
	// On the wire as well as in the row: the roster is what turns this id
	// back into a name, and a client that never receives it can't.
	if payload.Width != 2 || payload.Height != 2 {
		t.Errorf("broadcast size = %vx%v, want 2x2", payload.Width, payload.Height)
	}
	if payload.OwnerParticipantID == nil || *payload.OwnerParticipantID != r.player.ID {
		t.Errorf("broadcast owner = %v, want the player", payload.OwnerParticipantID)
	}

	stored, err := r.ts.store.GetToken(payload.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if stored.Width != 2 || stored.OwnerParticipantID == nil {
		t.Fatalf("stored token = %+v, want a 2x2 owned by the player", stored)
	}
}

// Most tokens are monsters: nobody owns them, and they stand in one
// square. Leaving both out has to mean that rather than being an error.
func TestTokenCreate_DefaultsToOneSquareAndUnowned(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.create", map[string]any{"sceneId": r.scene.ID, "name": "Goblin"})
	env := client.readEnvelope(t)
	if env.Type != "token.created" {
		t.Fatalf("type = %q, want token.created", env.Type)
	}

	tokens, err := r.ts.store.ListTokensForScene(r.scene.ID)
	if err != nil {
		t.Fatalf("ListTokensForScene: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("len(tokens) = %d, want 1", len(tokens))
	}
	if tokens[0].Width != 1 || tokens[0].Height != 1 {
		t.Errorf("size = %vx%v, want 1x1", tokens[0].Width, tokens[0].Height)
	}
	if tokens[0].OwnerParticipantID != nil {
		t.Errorf("owner = %v, want nobody", *tokens[0].OwnerParticipantID)
	}
}

// The same boundary requireAssetInRoom draws around art. A participant
// row exists globally, so "it's a real ID" is never the question —
// membership of this room is.
func TestTokenCreate_RefusesAnOwnerFromAnotherRoom(t *testing.T) {
	r := newTokenTestRoom(t)

	_, otherGM, err := r.ts.store.CreateRoom("Elsewhere", "Carol", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.create", map[string]any{
		"sceneId": r.scene.ID, "name": "Interloper", "ownerParticipantId": otherGM.ID,
	})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}

	tokens, err := r.ts.store.ListTokensForScene(r.scene.ID)
	if err != nil {
		t.Fatalf("ListTokensForScene: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("len(tokens) = %d, want the create refused outright", len(tokens))
	}
}

// A participant ID that belongs to nobody at all gets the same answer as
// one belonging to another room, so a refusal can't be used to sort real
// IDs from invented ones.
func TestTokenCreate_RefusesAnOwnerThatDoesNotExist(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.create", map[string]any{
		"sceneId": r.scene.ID, "name": "Ghost",
		"ownerParticipantId": "11111111-1111-1111-1111-111111111111",
	})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}

	tokens, err := r.ts.store.ListTokensForScene(r.scene.ID)
	if err != nil {
		t.Fatalf("ListTokensForScene: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("len(tokens) = %d, want 0", len(tokens))
	}
}
