package ws

import (
	"encoding/json"
	"testing"
	"time"

	"longtable/internal/store"
)

// The room's movement lock. Off, everything here is the open table
// Longtable has always been; on, a Player may drag only their own
// tokens. The GM is outside it either way.

func (r tokenTestRoom) lockMovement(t *testing.T) {
	t.Helper()

	if err := r.ts.store.SetOwnerOnlyMovement(r.room.ID, true); err != nil {
		t.Fatalf("SetOwnerOnlyMovement: %v", err)
	}
}

func (r tokenTestRoom) positionOf(t *testing.T, tokenID string) (float64, float64) {
	t.Helper()

	token, err := r.ts.store.GetToken(tokenID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	return token.X, token.Y
}

// The default, and the one that must not change: an open table.
func TestTokenMove_UnlockedRoomLetsAnyoneMoveAnything(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.move", map[string]any{"tokenId": token.ID, "x": 7, "y": 8})
	if env := client.readEnvelope(t); env.Type != "token.moved" {
		t.Fatalf("type = %q, want token.moved", env.Type)
	}

	if x, y := r.positionOf(t, token.ID); x != 7 || y != 8 {
		t.Fatalf("token at (%v, %v), want (7, 8)", x, y)
	}
}

func TestTokenMove_LockedRoomRefusesATokenAPlayerDoesNotOwn(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)
	r.lockMovement(t)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.move", map[string]any{"tokenId": token.ID, "x": 7, "y": 8})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}

	// Refused with no effect, which is the criterion: a move that fails
	// the check must not reach the store at all.
	if x, y := r.positionOf(t, token.ID); x != 3 || y != 4 {
		t.Fatalf("token at (%v, %v), want it left where it was", x, y)
	}
}

func TestTokenMove_LockedRoomStillLetsAPlayerMoveTheirOwn(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.ownedToken(t, "Bob's Fighter", store.VisibilityVisible, r.player.ID)
	r.lockMovement(t)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.move", map[string]any{"tokenId": token.ID, "x": 7, "y": 8})
	if env := client.readEnvelope(t); env.Type != "token.moved" {
		t.Fatalf("type = %q, want token.moved", env.Type)
	}
	if x, y := r.positionOf(t, token.ID); x != 7 || y != 8 {
		t.Fatalf("token at (%v, %v), want (7, 8)", x, y)
	}
}

// The GM is outside the lock they set — otherwise turning it on would
// take the monsters away from the only person who moves them.
func TestTokenMove_LockedRoomLeavesTheGMAlone(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.ownedToken(t, "Bob's Fighter", store.VisibilityVisible, r.player.ID)
	r.lockMovement(t)

	client := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.move", map[string]any{"tokenId": token.ID, "x": 1, "y": 2})
	if env := client.readEnvelope(t); env.Type != "token.moved" {
		t.Fatalf("type = %q, want token.moved", env.Type)
	}
}

// Same wording as token.update and token.delete: a hidden token doesn't
// exist as far as a Player is concerned, even one they own, or an
// ambush prepped with their own character announces itself.
func TestTokenMove_LockedRoomRefusesAHiddenTokenAsMissing(t *testing.T) {
	r := newTokenTestRoom(t)
	hidden := r.ownedToken(t, "Ambusher", store.VisibilityHidden, r.player.ID)
	r.lockMovement(t)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "token.move", map[string]any{"tokenId": hidden.ID, "x": 7, "y": 8})
	hiddenAnswer := errorMessage(t, client.readEnvelope(t))

	client.send(t, "token.move", map[string]any{
		"tokenId": "00000000-0000-4000-8000-000000000000", "x": 7, "y": 8,
	})
	missingAnswer := errorMessage(t, client.readEnvelope(t))

	if hiddenAnswer != missingAnswer {
		t.Fatalf("errors differ: %q vs %q — the two must be indistinguishable", hiddenAnswer, missingAnswer)
	}
}

// The setting is a room setting, so it reaches everyone the moment it
// changes — a Player whose next drag is about to be refused should have
// been told before they tried it.
func TestRoomSetOwnerOnlyMovement_ReachesTheWholeRoomAndPersists(t *testing.T) {
	r := newTokenTestRoom(t)

	gmClient := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gmClient.readEnvelope(t) // state.sync
	playerClient := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	playerClient.readEnvelope(t) // state.sync
	gmClient.readPresence(t)     // the player arriving

	gmClient.send(t, "room.setOwnerOnlyMovement", map[string]any{"ownerOnlyMovement": true})

	if env := gmClient.readEnvelope(t); env.Type != "room.updated" {
		t.Fatalf("gm event type = %q, want room.updated", env.Type)
	}
	env := playerClient.readEnvelope(t)
	if env.Type != "room.updated" {
		t.Fatalf("player event type = %q, want room.updated", env.Type)
	}

	var payload struct {
		Room struct {
			Name              string `json:"name"`
			OwnerOnlyMovement bool   `json:"ownerOnlyMovement"`
			GMPasswordHash    string `json:"gmPasswordHash"`
		} `json:"room"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal room.updated payload: %v", err)
	}
	if !payload.Room.OwnerOnlyMovement {
		t.Errorf("ownerOnlyMovement = false, want true")
	}
	// The whole room goes out, so it's worth asserting what doesn't: the
	// payload is built field by field precisely so a credential can't ride
	// along to every client in the room.
	if payload.Room.GMPasswordHash != "" {
		t.Errorf("room.updated carried a password hash")
	}
	if payload.Room.Name == "" {
		t.Errorf("room.updated carried no name, so it isn't the whole room")
	}

	room, err := r.ts.store.GetRoomByID(r.room.ID)
	if err != nil {
		t.Fatalf("GetRoomByID: %v", err)
	}
	if !room.OwnerOnlyMovement {
		t.Fatalf("stored room = %+v, want the setting persisted", room)
	}
}

func TestRoomSetOwnerOnlyMovement_PlayerMayNot(t *testing.T) {
	r := newTokenTestRoom(t)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	client.send(t, "room.setOwnerOnlyMovement", map[string]any{"ownerOnlyMovement": true})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}

	room, err := r.ts.store.GetRoomByID(r.room.ID)
	if err != nil {
		t.Fatalf("GetRoomByID: %v", err)
	}
	if room.OwnerOnlyMovement {
		t.Fatalf("a player turned the lock on")
	}
}

// A fresh client has to be told which way the lock is set, or it can't
// know which tokens to let go of.
func TestStateSync_CarriesTheMovementLock(t *testing.T) {
	r := newTokenTestRoom(t)
	r.lockMovement(t)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	env := client.readEnvelope(t)
	if env.Type != "state.sync" {
		t.Fatalf("type = %q, want state.sync", env.Type)
	}

	var payload struct {
		Room struct {
			OwnerOnlyMovement bool `json:"ownerOnlyMovement"`
		} `json:"room"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal state.sync payload: %v", err)
	}
	if !payload.Room.OwnerOnlyMovement {
		t.Fatalf("state.sync said the room was unlocked")
	}
}

// Undo sends an ordinary token.move, so the lock governs it with nothing
// extra written — the property the backlog item was counting on.
func TestTokenMove_LockedRoomAlsoRefusesTheUndoOfAMove(t *testing.T) {
	r := newTokenTestRoom(t)
	token := r.token(t, "Goblin", store.VisibilityVisible)

	client := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	client.readEnvelope(t) // state.sync

	// Moved while the table was open, which is what leaves an entry on
	// this session's history.
	client.send(t, "token.move", map[string]any{"tokenId": token.ID, "x": 7, "y": 8})
	if env := client.readEnvelope(t); env.Type != "token.moved" {
		t.Fatalf("type = %q, want token.moved", env.Type)
	}

	r.lockMovement(t)

	// The undo is the same command sent backwards, and it is refused now.
	client.send(t, "token.move", map[string]any{"tokenId": token.ID, "x": 3, "y": 4})
	if env := client.readEnvelope(t); env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
	if x, y := r.positionOf(t, token.ID); x != 7 || y != 8 {
		t.Fatalf("token at (%v, %v), want it left where the move put it", x, y)
	}

	client.expectNoMessage(t, 200*time.Millisecond)
}
