package ws

import (
	"encoding/json"
	"fmt"
	"testing"

	"longtable/internal/store"
)

// createdToken reads a token.created payload, for the fields these tests
// care about.
type createdToken struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	X                  float64 `json:"x"`
	Y                  float64 `json:"y"`
	OwnerParticipantID *string `json:"ownerParticipantId"`
	Visibility         string  `json:"visibility"`
}

func readCreatedToken(t *testing.T, c *testClient) createdToken {
	t.Helper()

	env := c.readEnvelope(t)
	if env.Type != "token.created" {
		t.Fatalf("type = %q, want token.created", env.Type)
	}
	var payload createdToken
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal token.created payload: %v", err)
	}
	return payload
}

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

// The headline of player-created tokens: a Player can make one at all,
// and it is theirs without their having chosen an owner.
func TestTokenCreate_PlayerOwnsWhatTheyMake(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.create", map[string]any{"sceneId": r.scene.ID, "name": "Familiar"})

	created := readCreatedToken(t, client)
	if created.OwnerParticipantID == nil || *created.OwnerParticipantID != r.player.ID {
		t.Fatalf("owner = %v, want the player who made it", created.OwnerParticipantID)
	}
}

// The two fields a Player doesn't get. Following token.update, they are
// ignored rather than refused — the values are preserved either way, and
// rejecting would turn a stale form into an error.
func TestTokenCreate_PlayerCannotHideOrGiveAway(t *testing.T) {
	r := newTokenTestRoom(t)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync
	gmClient.readPresence(t) // the player arriving

	client.send(t, "token.create", map[string]any{
		"sceneId": r.scene.ID, "name": "Ambusher",
		"visibility": "hidden", "ownerParticipantId": r.gm.ID,
	})

	created := readCreatedToken(t, client)
	if created.Visibility != string(store.VisibilityVisible) {
		t.Errorf("visibility = %q, want visible — hiding is a GM power", created.Visibility)
	}
	if created.OwnerParticipantID == nil || *created.OwnerParticipantID != r.player.ID {
		t.Errorf("owner = %v, want the creator rather than the one they asked for", created.OwnerParticipantID)
	}

	// The creator's own echo could be a lie about what was stored, so the
	// proof is that the GM's client — which would be the *only* recipient
	// of a genuinely hidden token — sees the same visible token.
	fromGM := readCreatedToken(t, gmClient)
	if fromGM.Visibility != string(store.VisibilityVisible) {
		t.Errorf("gm sees visibility %q, want visible", fromGM.Visibility)
	}
}

// Eight monkeys in one trip through the dialog, which is the whole point
// of the count.
func TestTokenCreate_CountNumbersThemAndSpreadsThemOut(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.create", map[string]any{
		"sceneId": r.scene.ID, "name": "Monkey", "x": 4, "y": 4, "count": 8,
	})

	seen := map[cell]bool{}
	for i := 1; i <= 8; i++ {
		created := readCreatedToken(t, client)
		if want := fmt.Sprintf("Monkey %d", i); created.Name != want {
			t.Errorf("name = %q, want %q", created.Name, want)
		}
		at := cell{X: created.X, Y: created.Y}
		if seen[at] {
			t.Errorf("%s landed on (%v, %v), which is already taken", created.Name, at.X, at.Y)
		}
		seen[at] = true
	}

	if got := r.tokenCount(t); got != 8 {
		t.Fatalf("len(tokens) = %d, want 8", got)
	}
}

// No suffix on a single token, or every one a GM makes picks up a
// pointless " 1".
func TestTokenCreate_ACountOfOneIsNamedExactlyWhatWasTyped(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.create", map[string]any{
		"sceneId": r.scene.ID, "name": "Monkey", "count": 1,
	})
	if created := readCreatedToken(t, client); created.Name != "Monkey" {
		t.Fatalf("name = %q, want %q", created.Name, "Monkey")
	}
}

// The stepper is a convenience, not a permission.
func TestTokenCreate_RefusesMoreThanTheCap(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	for _, count := range []int{maxTokensPerCreate + 1, 500, -1} {
		client.send(t, "token.create", map[string]any{
			"sceneId": r.scene.ID, "name": "Monkey", "count": count,
		})
		if env := client.readEnvelope(t); env.Type != "error" {
			t.Fatalf("count %d: type = %q, want error", count, env.Type)
		}
	}

	if got := r.tokenCount(t); got != 0 {
		t.Fatalf("len(tokens) = %d, want none of them created", got)
	}
}

// A short id list would leave the client holding ids for tokens that
// came back under different ones, so its undo entries would point at
// nothing. Refusing is kinder than half-matching.
func TestTokenCreate_RefusesIDsThatDoNotMatchTheCount(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.create", map[string]any{
		"sceneId": r.scene.ID, "name": "Monkey", "count": 3,
		"tokenIds": []string{"11111111-1111-4111-8111-111111111111"},
	})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
	if got := r.tokenCount(t); got != 0 {
		t.Fatalf("len(tokens) = %d, want 0", got)
	}
}

// The whole batch comes back under the ids the client minted, in order —
// which is what lets it record one undo entry per token without having
// to guess which of the arriving echoes are its own.
func TestTokenCreate_UsesTheClientsIDsInOrder(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	ids := []string{
		"11111111-1111-4111-8111-111111111111",
		"22222222-2222-4222-8222-222222222222",
		"33333333-3333-4333-8333-333333333333",
	}
	client.send(t, "token.create", map[string]any{
		"sceneId": r.scene.ID, "name": "Monkey", "count": 3, "tokenIds": ids,
	})

	for i, want := range ids {
		if got := readCreatedToken(t, client).ID; got != want {
			t.Fatalf("token %d: id = %q, want %q", i+1, got, want)
		}
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
