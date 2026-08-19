package ws

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"longtable/internal/store"
)

// Deleting a token is the first thing that removes a *token* rather than
// a drawing, and undoing it recreates one under an id the client chose —
// so what these cover is the permission boundary, the room boundary, and
// the round trip that makes undo possible at all.

type tokenTestRoom struct {
	ts     *testServer
	room   store.Room
	gm     store.Participant
	player store.Participant
	scene  store.Scene
}

func newTokenTestRoom(t *testing.T) tokenTestRoom {
	t.Helper()

	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	player, err := ts.store.JoinRoom(room.ID, "Bob", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	scene, err := ts.store.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	return tokenTestRoom{ts: ts, room: room, gm: gm, player: player, scene: scene}
}

func (r tokenTestRoom) token(t *testing.T, name string, visibility store.Visibility) store.Token {
	t.Helper()

	token, err := r.ts.store.CreateToken(store.Token{
		SceneID: r.scene.ID, Name: name, X: 3, Y: 4, Width: 2, Height: 2, Visibility: visibility,
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	return token
}

func (r tokenTestRoom) tokenCount(t *testing.T) int {
	t.Helper()

	tokens, err := r.ts.store.ListTokensForScene(r.scene.ID)
	if err != nil {
		t.Fatalf("ListTokensForScene: %v", err)
	}
	return len(tokens)
}

func deletedTokenID(t *testing.T, env envelope) string {
	t.Helper()

	var payload struct {
		TokenID string `json:"tokenId"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal token.deleted payload: %v", err)
	}
	return payload.TokenID
}

func TestTokenDelete_GMRemovesItForTheWholeRoom(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync

	gmClient.send(t, "token.delete", map[string]any{"tokenId": token.ID})

	// The sender gets its own echo first; the player's copy is the one
	// that proves the whole room heard.
	if env := gmClient.readEnvelope(t); env.Type != "token.deleted" {
		t.Fatalf("gm event type = %q, want token.deleted", env.Type)
	}
	env := playerClient.readEnvelope(t)
	if env.Type != "token.deleted" {
		t.Fatalf("player event type = %q, want token.deleted", env.Type)
	}
	if got := deletedTokenID(t, env); got != token.ID {
		t.Fatalf("deleted tokenId = %q, want %q", got, token.ID)
	}

	if got := r.tokenCount(t); got != 0 {
		t.Fatalf("len(tokens) = %d, want 0 (the deletion must persist)", got)
	}
}

// Clearing away your own summons is the other half of being able to
// make them: without it, eight conjured monkeys become the GM's cleanup.
func TestTokenDelete_PlayerRemovesTheirOwn(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.ownedToken(t, "Monkey", store.VisibilityVisible, r.player.ID)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.delete", map[string]any{"tokenId": token.ID})
	if env := client.readEnvelope(t); env.Type != "token.deleted" {
		t.Fatalf("type = %q, want token.deleted", env.Type)
	}
	if got := r.tokenCount(t); got != 0 {
		t.Fatalf("len(tokens) = %d, want 0", got)
	}
}

// Ownership is the whole rule: a monster nobody owns is still the GM's
// to remove, and a Player deleting one would be deleting the scene.
func TestTokenDelete_PlayerMayNotRemoveATokenTheyDoNotOwn(t *testing.T) {
	r := newTokenTestRoom(t)
	unowned := r.token(t, "Goblin", store.VisibilityVisible)
	someoneElses := r.ownedToken(t, "Alice's Fighter", store.VisibilityVisible, r.gm.ID)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	for _, token := range []store.Token{unowned, someoneElses} {
		client.send(t, "token.delete", map[string]any{"tokenId": token.ID})
		if env := client.readEnvelope(t); env.Type != "error" {
			t.Fatalf("%s: type = %q, want error", token.Name, env.Type)
		}
	}

	if got := r.tokenCount(t); got != 2 {
		t.Fatalf("len(tokens) = %d, want 2 (neither may be deleted)", got)
	}
}

// A hidden token is refused in the words of one that isn't there, even
// to its own owner — a GM can prep an ambush with a Player's character,
// and an error separating "not yours" from "no such token" is how they'd
// find out. Same rule, same wording, as token.update.
func TestTokenDelete_HiddenTokenIsRefusedToItsOwnerAsMissing(t *testing.T) {
	r := newTokenTestRoom(t)
	hidden := r.ownedToken(t, "Ambusher", store.VisibilityHidden, r.player.ID)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.delete", map[string]any{"tokenId": hidden.ID})
	hiddenAnswer := errorMessage(t, client.readEnvelope(t))

	client.send(t, "token.delete", map[string]any{"tokenId": "00000000-0000-4000-8000-000000000000"})
	missingAnswer := errorMessage(t, client.readEnvelope(t))

	if hiddenAnswer != missingAnswer {
		t.Fatalf("errors differ: %q vs %q — the two must be indistinguishable", hiddenAnswer, missingAnswer)
	}
	if got := r.tokenCount(t); got != 1 {
		t.Fatalf("len(tokens) = %d, want 1", got)
	}
}

// A token id from another room has to fail exactly as an id that doesn't
// exist does, or the difference is a way to probe for what exists
// elsewhere on the server.
func TestTokenDelete_TokenFromAnotherRoomFailsLikeAMissingOne(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	otherRoom, otherGM, err := r.ts.store.CreateRoom("Other", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	client := r.ts.connect(t, otherRoom.Slug, otherGM.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.delete", map[string]any{"tokenId": token.ID})
	fromOtherRoom := errorMessage(t, client.readEnvelope(t))

	client.send(t, "token.delete", map[string]any{"tokenId": "00000000-0000-4000-8000-000000000000"})
	fromMissing := errorMessage(t, client.readEnvelope(t))

	if fromOtherRoom != fromMissing {
		t.Fatalf("errors differ: %q vs %q — the two must be indistinguishable", fromOtherRoom, fromMissing)
	}

	if got := r.tokenCount(t); got != 1 {
		t.Fatalf("len(tokens) = %d, want 1 (another room's GM must not be able to delete it)", got)
	}
}

func TestTokenDelete_RejectsMalformedPayload(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.delete", map[string]any{})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
}

// Players were never told a hidden token existed, so they must not be
// told it's gone either — an id they've never seen turning up in a
// deletion is itself the leak.
func TestTokenDelete_HiddenTokenDeletionWithheldFromPlayers(t *testing.T) {
	r := newTokenTestRoom(t)
	hidden := r.token(t, "Ambusher", store.VisibilityHidden)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync

	gmClient.send(t, "token.delete", map[string]any{"tokenId": hidden.ID})
	if env := gmClient.readEnvelope(t); env.Type != "token.deleted" {
		t.Fatalf("gm event type = %q, want token.deleted", env.Type)
	}

	playerClient.expectNoMessage(t, 200*time.Millisecond)
}

// Undo is a token.create carrying the id the client already knows, so
// the token comes back as the same token to everyone still holding it
// rather than as a new one that merely looks the same.
func TestTokenCreate_UsesClientSuppliedIDSoADeleteCanBeUndone(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.delete", map[string]any{"tokenId": token.ID})
	if env := client.readEnvelope(t); env.Type != "token.deleted" {
		t.Fatalf("type = %q, want token.deleted", env.Type)
	}

	client.send(t, "token.create", map[string]any{
		"tokenIds": []string{token.ID},
		"sceneId":  r.scene.ID,
		"name":     token.Name,
		"x":        token.X,
		"y":        token.Y,
		"width":    token.Width,
		"height":   token.Height,
	})
	env := client.readEnvelope(t)
	if env.Type != "token.created" {
		t.Fatalf("type = %q, want token.created", env.Type)
	}

	var payload struct {
		ID     string  `json:"id"`
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal token.created payload: %v", err)
	}
	if payload.ID != token.ID {
		t.Fatalf("id = %q, want the client's %q", payload.ID, token.ID)
	}
	if payload.X != token.X || payload.Y != token.Y {
		t.Fatalf("restored at (%v, %v), want (%v, %v)", payload.X, payload.Y, token.X, token.Y)
	}
	if payload.Width != token.Width || payload.Height != token.Height {
		t.Fatalf("restored %vx%v, want %vx%v", payload.Width, payload.Height, token.Width, token.Height)
	}

	stored, err := r.ts.store.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken after undo: %v", err)
	}
	if stored.SceneID != r.scene.ID {
		t.Fatalf("restored under the wrong scene: %+v", stored)
	}
}

func TestTokenCreate_RejectsMalformedID(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	// Only the canonical spelling: uuid.Parse also takes the braced and
	// uppercase forms, which would come back as a different string from
	// the one the client sent.
	for _, id := range []string{
		"not-a-uuid",
		"{6f1e3b8a-2c4d-4f1e-9a7b-0d5c8e2f4a13}",
		"6F1E3B8A-2C4D-4F1E-9A7B-0D5C8E2F4A13",
	} {
		client.send(t, "token.create", map[string]any{
			"tokenIds": []string{id}, "sceneId": r.scene.ID, "name": "Goblin",
		})
		if env := client.readEnvelope(t); env.Type != "error" {
			t.Fatalf("tokenId %q: type = %q, want error", id, env.Type)
		}
	}

	if got := r.tokenCount(t); got != 0 {
		t.Fatalf("len(tokens) = %d, want 0", got)
	}
}

// The store's own contract, relied on above: a second delete of the same
// row is a no-op rather than an error, so two GMs racing on one token
// don't fail the slower one.
func TestStore_DeleteTokenIsIdempotentAndGetTokenReportsMissing(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	if err := r.ts.store.DeleteToken(token.ID); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}
	if err := r.ts.store.DeleteToken(token.ID); err != nil {
		t.Fatalf("DeleteToken (second time): %v", err)
	}
	if _, err := r.ts.store.GetToken(token.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetToken after delete = %v, want ErrNotFound", err)
	}
}
