package store

import (
	"errors"
	"testing"
)

func partyRooms(t *testing.T) (s *Store, roomID, otherRoomID string) {
	t.Helper()
	s = newTestStore(t)
	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	other, _, err := s.CreateRoom("Other", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom(other): %v", err)
	}
	return s, room.ID, other.ID
}

func mustCreateMember(t *testing.T, s *Store, roomID, name string) PartyMember {
	t.Helper()
	m, err := s.CreatePartyMember(PartyMember{RoomID: roomID, Name: name, Width: 1, Height: 1})
	if err != nil {
		t.Fatalf("CreatePartyMember(%s): %v", name, err)
	}
	return m
}

func names(members []PartyMember) []string {
	out := make([]string, len(members))
	for i, m := range members {
		out[i] = m.Name
	}
	return out
}

// Listed in the order they were added — the order the dialog shows them
// and party.add puts them down — and never another room's.
func TestListPartyMembers_InTheOrderAddedAndOnlyThisRoom(t *testing.T) {
	s, roomID, otherID := partyRooms(t)
	for _, n := range []string{"Aria", "Brom", "Cyl"} {
		mustCreateMember(t, s, roomID, n)
	}
	mustCreateMember(t, s, otherID, "Stranger")

	members, err := s.ListPartyMembers(roomID)
	if err != nil {
		t.Fatalf("ListPartyMembers: %v", err)
	}
	if got := names(members); len(got) != 3 || got[0] != "Aria" || got[1] != "Brom" || got[2] != "Cyl" {
		t.Fatalf("members = %v, want [Aria Brom Cyl]", got)
	}
	if n, err := s.CountPartyMembers(roomID); err != nil || n != 3 {
		t.Fatalf("CountPartyMembers = %d, %v; want 3", n, err)
	}
}

// An empty party is an empty list, not nil, so it reaches the client as
// [] rather than null.
func TestListPartyMembers_EmptyIsNotNil(t *testing.T) {
	s, roomID, _ := partyRooms(t)
	members, err := s.ListPartyMembers(roomID)
	if err != nil || members == nil {
		t.Fatalf("ListPartyMembers = %v, %v; want an empty list", members, err)
	}
}

func TestUpdatePartyMember_ChangesTheMember(t *testing.T) {
	s, roomID, _ := partyRooms(t)
	m := mustCreateMember(t, s, roomID, "Aria")

	m.Name, m.Width, m.Height = "Aria the Bold", 2, 2
	if err := s.UpdatePartyMember(m); err != nil {
		t.Fatalf("UpdatePartyMember: %v", err)
	}
	members, _ := s.ListPartyMembers(roomID)
	if len(members) != 1 || members[0].Name != "Aria the Bold" || members[0].Width != 2 {
		t.Fatalf("members = %+v", members)
	}
}

// A member id from another room answers exactly as a missing one does,
// for edits and deletes both — so one room can't reach, or even probe
// for, another's party.
func TestPartyMember_WritesAreScopedToTheRoom(t *testing.T) {
	s, roomID, otherID := partyRooms(t)
	m := mustCreateMember(t, s, roomID, "Aria")

	elsewhere := m
	elsewhere.RoomID = otherID
	elsewhere.Name = "Hijacked"
	if err := s.UpdatePartyMember(elsewhere); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update from another room: %v, want ErrNotFound", err)
	}
	if err := s.DeletePartyMember(otherID, m.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete from another room: %v, want ErrNotFound", err)
	}
	if err := s.DeletePartyMember(roomID, "no-such-member"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete of a missing member: %v, want ErrNotFound", err)
	}

	members, _ := s.ListPartyMembers(roomID)
	if len(members) != 1 || members[0].Name != "Aria" {
		t.Fatalf("members = %+v, want Aria untouched", members)
	}
}

// Leaving the party isn't leaving the scene: a member's copies stay on
// their maps, still carrying the key, which now matches nothing.
func TestDeletePartyMember_LeavesItsCopiesOnTheMap(t *testing.T) {
	s, roomID, _ := partyRooms(t)
	scene, err := s.CreateScene(roomID, "Corridor", nil, 70, 700, 700)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	m := mustCreateMember(t, s, roomID, "Aria")
	copied, err := s.CreateToken(Token{SceneID: scene.ID, Name: "Aria", Width: 1, Height: 1, PartyKey: m.ID})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	if err := s.DeletePartyMember(roomID, m.ID); err != nil {
		t.Fatalf("DeletePartyMember: %v", err)
	}
	if _, err := s.GetToken(copied.ID); err != nil {
		t.Fatalf("the copy went with the member: %v", err)
	}
}

// updated_at was written while the party was the newest copy of a marked
// token, and was dropped when it moved into its own table. A database
// that got the column has it taken out again; one that never did is left
// alone — both on every start.
func TestNew_DropsADeadColumnOnlyWhenItIsThere(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.db.Exec(`ALTER TABLE token ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''`); err != nil {
		t.Fatalf("add the old column back: %v", err)
	}

	for range 2 {
		if err := s.dropDeadColumns(); err != nil {
			t.Fatalf("dropDeadColumns: %v", err)
		}
		has, err := s.hasColumn("token", "updated_at")
		if err != nil {
			t.Fatalf("hasColumn: %v", err)
		}
		if has {
			t.Fatal("token.updated_at survived")
		}
	}
}

// Attendance is stored, so a night's ticks hold for every scene that
// night and survive a reload.
func TestPartyMember_KeepsWhetherTheyAreAttending(t *testing.T) {
	s, roomID, _ := partyRooms(t)
	m, err := s.CreatePartyMember(PartyMember{RoomID: roomID, Name: "Aria", Width: 1, Height: 1, Attending: true})
	if err != nil {
		t.Fatalf("CreatePartyMember: %v", err)
	}
	m.Attending = false
	if err := s.UpdatePartyMember(m); err != nil {
		t.Fatalf("UpdatePartyMember: %v", err)
	}
	members, _ := s.ListPartyMembers(roomID)
	if members[0].Attending {
		t.Fatal("attending came back true after being turned off")
	}
}
