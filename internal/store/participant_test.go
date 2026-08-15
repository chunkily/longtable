package store

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

func TestJoinRoom(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	p, err := s.JoinRoom(room.ID, "Bob", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	if p.Role != RolePlayer {
		t.Fatalf("role = %q, want player", p.Role)
	}

	got, err := s.GetParticipantByToken(room.ID, p.SessionToken)
	if err != nil {
		t.Fatalf("GetParticipantByToken: %v", err)
	}
	if got.ID != p.ID {
		t.Fatalf("resolved participant %q, want %q", got.ID, p.ID)
	}
}

func TestGetParticipantByToken_ScopedToRoom(t *testing.T) {
	s := newTestStore(t)

	roomA, _, err := s.CreateRoom("Room A", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	roomB, _, err := s.CreateRoom("Room B", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	p, err := s.JoinRoom(roomA.ID, "Bob", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	if _, err := s.GetParticipantByToken(roomB.ID, p.SessionToken); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound (token must not resolve in a different room)", err)
	}
}

func TestGMLogin(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	p, err := s.GMLogin(room.ID, "Second GM", "")
	if err != nil {
		t.Fatalf("GMLogin: %v", err)
	}
	if p.Role != RoleGM {
		t.Fatalf("role = %q, want gm", p.Role)
	}
}

// A colour belongs to the seat, which is what makes it survive the
// device that chose it — the criterion the story could not satisfy
// before seats and sessions were separated.
func TestIdentityColor_BelongsToTheSeatRatherThanTheDevice(t *testing.T) {
	s := newTestStore(t)
	room, _, err := s.CreateRoom("Room", "GM", "violet", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	bob, err := s.JoinRoom(room.ID, "Bob", "teal")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	if bob.Color != "teal" {
		t.Fatalf("Color = %q, want teal", bob.Color)
	}

	// A different device taking the same seat gets the seat's colour, not
	// a blank one: this is the "comes back on another device" criterion.
	returning, err := s.ClaimSeat(room.ID, bob.ID)
	if err != nil {
		t.Fatalf("ClaimSeat: %v", err)
	}
	if returning.Color != "teal" {
		t.Fatalf("reclaimed Color = %q, want teal", returning.Color)
	}
	if returning.SessionToken == bob.SessionToken {
		t.Fatal("claiming should mint a new session, so this test isn't reading the same device back")
	}

	// And the GM's, which is created down a different path entirely.
	participants, err := s.ListParticipantsForRoom(room.ID)
	if err != nil {
		t.Fatalf("ListParticipantsForRoom: %v", err)
	}
	for _, p := range participants {
		if p.Role == RoleGM && p.Color != "violet" {
			t.Fatalf("GM Color = %q, want violet", p.Color)
		}
	}
}

// Two seats may hold the same colour. The picker shows what is taken so
// somebody can avoid a clash if they want one avoided; nothing enforces
// it, because a room that refuses a colour is a room arguing with the
// people at the table.
func TestIdentityColor_TwoSeatsMayShareOne(t *testing.T) {
	s := newTestStore(t)
	room, _, err := s.CreateRoom("Room", "GM", "green", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	if _, err := s.JoinRoom(room.ID, "Bob", "green"); err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	if _, err := s.JoinRoom(room.ID, "Carol", "green"); err != nil {
		t.Fatalf("second JoinRoom with the same colour: %v", err)
	}
}

// Seats made before colours existed read as unchosen rather than as
// anything the client has to guess at.
func TestIdentityColor_EmptyIsValidAndMeansUnchosen(t *testing.T) {
	if !ValidIdentityColor("") {
		t.Error("empty should be valid: every seat made before this feature has it")
	}
	if !ValidIdentityColor("violet") {
		t.Error("violet is in the palette and should be valid")
	}
	// A key the palette used to carry. Retiring one has to be refused on
	// the way in, or a seat could be written into a colour nothing can
	// render.
	if ValidIdentityColor("lime") {
		t.Error("lime was retired from the palette and should no longer be accepted")
	}
	if ValidIdentityColor("rgb(1,2,3)") || ValidIdentityColor("crimson") {
		t.Error("anything outside the palette must be refused — this value reaches a style attribute")
	}
}

// The palette lives twice: these keys here, and the hex each one is
// painted in over in web/src/lib/identity-color.ts. A key on one side
// with no partner on the other is a seat whose colour renders as nothing
// — silently, for one person, on a screen nobody testing Go will look
// at. Reading the other file is cheap; discovering that at a table is
// not.
func TestIdentityColors_MatchTheClientPalette(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "lib", "identity-color.ts"))
	if err != nil {
		t.Fatalf("read the client palette: %v", err)
	}

	// The hyphen in the class is deliberate slack. Keys are single words
	// today (`red`, for a swatch called Blood Red), and a pattern that
	// stopped at a hyphen would silently truncate the first one that
	// isn't — reporting a mismatch that was really this regex's fault.
	found := regexp.MustCompile(`key:\s*'([a-z-]+)'`).FindAllStringSubmatch(string(source), -1)
	client := make([]string, 0, len(found))
	for _, m := range found {
		client = append(client, m[1])
	}

	if !slices.Equal(client, IdentityColors) {
		t.Errorf("client palette %v, server palette %v — they have to hold the same keys in the same order", client, IdentityColors)
	}
}
