package ws

import (
	"context"
	"encoding/json"
	"testing"
)

// The join password is set over REST, not a command — see
// internal/api/rooms.go's setJoinPassword — but whether one is set
// reaches the room the same way room.setOwnerOnlyMovement's result does:
// BroadcastRoomUpdated is what the REST handler calls in place of
// broadcasting from inside a command handler.
func TestBroadcastRoomUpdated_CarriesWhetherAJoinPasswordIsSet(t *testing.T) {
	r := newTokenTestRoom(t)

	gm := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gm.readEnvelope(t) // state.sync

	if err := r.ts.store.SetJoinPassword(r.room.ID, "letmein"); err != nil {
		t.Fatalf("SetJoinPassword: %v", err)
	}
	room, err := r.ts.store.GetRoomByID(r.room.ID)
	if err != nil {
		t.Fatalf("GetRoomByID: %v", err)
	}
	r.ts.hub.BroadcastRoomUpdated(context.Background(), room)

	env := gm.readEnvelope(t)
	if env.Type != "room.updated" {
		t.Fatalf("type = %q, want room.updated", env.Type)
	}
	var payload struct {
		Room struct {
			JoinPasswordSet bool `json:"joinPasswordSet"`
		} `json:"room"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if !payload.Room.JoinPasswordSet {
		t.Error("joinPasswordSet = false after setting a join password")
	}
}
