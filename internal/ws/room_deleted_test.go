package ws

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// A room can end under the people sitting in it, and the socket is the
// only thing that can tell them so. This is the one broadcast the hub
// makes on someone else's behalf — the REST handler does the deleting —
// so what it has to prove is that the news arrives and the connection
// then ends.

func TestRoomDeleted_ReachesEveryoneAndClosesTheirSockets(t *testing.T) {
	r := newTokenTestRoom(t)

	gm := r.ts.connect(t, r.room.Slug, r.gm.SessionToken)
	gm.readEnvelope(t) // state.sync
	player := r.ts.connect(t, r.room.Slug, r.player.SessionToken)
	player.readEnvelope(t) // state.sync

	// Deleted the way the REST handler does it: the row first, so nothing
	// can rejoin into the gap, and the room told afterwards.
	if err := r.ts.store.DeleteRoom(r.room.ID); err != nil {
		t.Fatalf("DeleteRoom: %v", err)
	}
	r.ts.hub.RoomDeleted(context.Background(), r.room.ID)

	for name, c := range map[string]*testClient{"the GM": gm, "the player": player} {
		if env := c.readEnvelope(t); env.Type != "room.deleted" {
			t.Fatalf("%s: type = %q, want room.deleted", name, env.Type)
		}

		// And then the socket goes, so a client that somehow missed the
		// event still stops rather than retrying a room that isn't there.
		//
		// Read until it does rather than expecting the very next read to
		// fail: closing one socket in a room broadcasts a measure.ended for
		// it to everyone else, so whichever client is checked second has
		// that in front of its own close.
		if err := readUntilClosed(c); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// Rejoining is a REST concern, but the socket is the other door into a
// room and it has to be shut too: a client holding a session token for a
// room that no longer exists must not get back in.
func TestRoomDeleted_RefusesAFreshConnection(t *testing.T) {
	r := newTokenTestRoom(t)

	if err := r.ts.store.DeleteRoom(r.room.ID); err != nil {
		t.Fatalf("DeleteRoom: %v", err)
	}
	r.ts.hub.RoomDeleted(context.Background(), r.room.ID)

	if err := r.ts.dialErr(r.room.Slug, r.gm.SessionToken); err == nil {
		t.Fatal("dialling a deleted room succeeded, want the upgrade refused")
	}
}

// readUntilClosed drains whatever is left on a socket and reports
// whether it ended within the timeout.
func readUntilClosed(c *testClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for {
		if _, _, err := c.conn.Read(ctx); err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("socket still open after the room was deleted")
			}
			return nil
		}
	}
}
