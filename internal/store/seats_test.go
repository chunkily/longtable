package store

import (
	"errors"
	"testing"
)

// The case that proves a seat isn't a renamed session: two devices, one
// seat, one identity.
func TestClaimSeat_SecondDeviceGetsTheSameSeat(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	bob, err := s.JoinRoom(room.ID, "Bob")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	second, err := s.ClaimSeat(room.ID, bob.ID)
	if err != nil {
		t.Fatalf("ClaimSeat: %v", err)
	}
	if second.ID != bob.ID {
		t.Fatalf("claim made a new seat %q, want %q", second.ID, bob.ID)
	}
	if second.SessionToken == bob.SessionToken {
		t.Fatal("claim reused the first device's token; each device gets its own")
	}

	// Both tokens resolve to one seat, so the roster has one Bob.
	for _, token := range []string{bob.SessionToken, second.SessionToken} {
		p, err := s.GetParticipantByToken(room.ID, token)
		if err != nil {
			t.Fatalf("GetParticipantByToken: %v", err)
		}
		if p.ID != bob.ID {
			t.Errorf("token resolved to %q, want the one seat %q", p.ID, bob.ID)
		}
	}

	participants, err := s.ListParticipantsForRoom(room.ID)
	if err != nil {
		t.Fatalf("ListParticipantsForRoom: %v", err)
	}
	if len(participants) != 2 { // the GM's seat and Bob's
		t.Fatalf("roster has %d entries, want 2 — a second device must not add a person", len(participants))
	}
}

// "Taking a seat I had before restores the tokens I own" needs no
// migration of token rows: they point at the participant already, and
// that row is the seat. Worth asserting rather than assuming, since it
// is the whole benefit of the split.
func TestClaimSeat_KeepsTheTokensTheSeatOwns(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	bob, err := s.JoinRoom(room.ID, "Bob")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	token, err := s.CreateToken(Token{
		SceneID: scene.ID, Name: "Bob's fighter", X: 1, Y: 1, Width: 1, Height: 1,
		OwnerParticipantID: &bob.ID, Visibility: VisibilityVisible,
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	// A new device, no stored session, taking the seat back.
	if _, err := s.ClaimSeat(room.ID, bob.ID); err != nil {
		t.Fatalf("ClaimSeat: %v", err)
	}

	got, err := s.GetToken(token.ID)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if got.OwnerParticipantID == nil || *got.OwnerParticipantID != bob.ID {
		t.Fatalf("token owner = %v, want the reclaimed seat %q", got.OwnerParticipantID, bob.ID)
	}
}

// Signing one device out leaves the seat and every other device on it
// alone. That's what makes leaving a room cheap.
func TestDeleteSession_LeavesTheSeatAndOtherDevices(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	bob, err := s.JoinRoom(room.ID, "Bob")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	second, err := s.ClaimSeat(room.ID, bob.ID)
	if err != nil {
		t.Fatalf("ClaimSeat: %v", err)
	}

	if err := s.DeleteSession(bob.SessionToken); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	if _, err := s.GetParticipantByToken(room.ID, bob.SessionToken); !errors.Is(err, ErrNotFound) {
		t.Fatalf("signed-out token still resolves: %v", err)
	}
	if _, err := s.GetParticipantByToken(room.ID, second.SessionToken); err != nil {
		t.Fatalf("the other device was signed out too: %v", err)
	}
	if ok, _ := s.ParticipantInRoom(room.ID, bob.ID); !ok {
		t.Fatal("the seat went with the session; leaving must cost a session, not an identity")
	}
}

func TestClaimSeat_RefusesTheGMSeatAndAnotherRoom(t *testing.T) {
	s := newTestStore(t)

	room, gm, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	other, _, err := s.CreateRoom("Other", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	bob, err := s.JoinRoom(room.ID, "Bob")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	// The GM seat is a role boundary: it goes through the password.
	if _, err := s.ClaimSeat(room.ID, gm.ID); !errors.Is(err, ErrGMSeatNeedsPassword) {
		t.Fatalf("claiming the GM seat: err = %v, want ErrGMSeatNeedsPassword", err)
	}
	// A seat is scoped to its room, like every other id in this store.
	if _, err := s.ClaimSeat(other.ID, bob.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("claiming across rooms: err = %v, want ErrNotFound", err)
	}
}

// A GM logging in from a second device takes the same GM seat rather
// than minting another one — before seats, every login grew the roster.
func TestGMLogin_ReusesTheGMSeat(t *testing.T) {
	s := newTestStore(t)

	room, gm, err := s.CreateRoom("Room", "Alice", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	again, err := s.GMLogin(room.ID, "Alice on her phone")
	if err != nil {
		t.Fatalf("GMLogin: %v", err)
	}
	if again.ID != gm.ID {
		t.Fatalf("GM login made a new seat %q, want %q", again.ID, gm.ID)
	}
	if again.SessionToken == gm.SessionToken {
		t.Fatal("expected a fresh session token for the second device")
	}

	participants, err := s.ListParticipantsForRoom(room.ID)
	if err != nil {
		t.Fatalf("ListParticipantsForRoom: %v", err)
	}
	if len(participants) != 1 {
		t.Fatalf("roster has %d entries, want 1", len(participants))
	}
}

func TestSeats_ListedForThePreJoinScreenWithoutCredentials(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "Alice", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	bob, err := s.JoinRoom(room.ID, "Bob")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	if _, err := s.ClaimSeat(room.ID, bob.ID); err != nil {
		t.Fatalf("ClaimSeat: %v", err)
	}
	// A seat a GM set up before anyone arrived: nobody has signed in.
	empty, err := s.CreateSeat(room.ID, "Carol")
	if err != nil {
		t.Fatalf("CreateSeat: %v", err)
	}

	seats, err := s.ListSeatsForRoom(room.ID)
	if err != nil {
		t.Fatalf("ListSeatsForRoom: %v", err)
	}
	if len(seats) != 3 {
		t.Fatalf("len(seats) = %d, want 3", len(seats))
	}

	byID := map[string]Seat{}
	for _, seat := range seats {
		byID[seat.ID] = seat
	}
	if got := byID[bob.ID].Sessions; got != 2 {
		t.Errorf("Bob's seat has %d sessions, want 2 (two devices)", got)
	}
	if got := byID[empty.ID].Sessions; got != 0 {
		t.Errorf("an unclaimed seat has %d sessions, want 0", got)
	}
	if byID[empty.ID].Role != RolePlayer {
		t.Errorf("a GM-added seat should be a player seat, got %q", byID[empty.ID].Role)
	}
}

func TestDeleteSeat_RemovesSessionsButRefusesTheGM(t *testing.T) {
	s := newTestStore(t)

	room, gm, err := s.CreateRoom("Room", "Alice", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	bob, err := s.JoinRoom(room.ID, "Bob")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	if err := s.DeleteSeat(room.ID, gm.ID); !errors.Is(err, ErrCannotDeleteGMSeat) {
		t.Fatalf("deleting the GM seat: err = %v, want ErrCannotDeleteGMSeat", err)
	}

	if err := s.DeleteSeat(room.ID, bob.ID); err != nil {
		t.Fatalf("DeleteSeat: %v", err)
	}
	if ok, _ := s.ParticipantInRoom(room.ID, bob.ID); ok {
		t.Fatal("seat survived deletion")
	}
	// The session went with it, by cascade.
	if _, err := s.GetParticipantByToken(room.ID, bob.SessionToken); !errors.Is(err, ErrNotFound) {
		t.Fatalf("session outlived its seat: %v", err)
	}
}
