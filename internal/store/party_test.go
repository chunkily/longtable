package store

import (
	"slices"
	"testing"
)

// partyFixture is a room with two scenes and a second room beside it, so
// every party query has something on another scene and something in
// another room that it must and must not see.
func partyFixture(t *testing.T) (s *Store, roomID string, first, second Scene, otherRoomScene Scene) {
	t.Helper()
	s = newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	first, err = s.CreateScene(room.ID, "Corridor", nil, 70, 700, 700)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	second, err = s.CreateScene(room.ID, "Boss room", nil, 70, 700, 700)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	other, _, err := s.CreateRoom("Other", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom(other): %v", err)
	}
	otherRoomScene, err = s.CreateScene(other.ID, "Elsewhere", nil, 70, 700, 700)
	if err != nil {
		t.Fatalf("CreateScene(other): %v", err)
	}
	return s, room.ID, first, second, otherRoomScene
}

func mustCreateToken(t *testing.T, s *Store, tok Token) Token {
	t.Helper()
	if tok.Width == 0 {
		tok.Width, tok.Height = 1, 1
	}
	created, err := s.CreateToken(tok)
	if err != nil {
		t.Fatalf("CreateToken(%s): %v", tok.Name, err)
	}
	return created
}

func memberNames(tokens []Token) []string {
	names := make([]string, len(tokens))
	for i, tok := range tokens {
		names[i] = tok.Name
	}
	slices.Sort(names)
	return names
}

// One per character however many copies exist — that's the whole reason
// the party is a key rather than a flag — and never another room's, even
// though keys are just strings and nothing stops two rooms holding one.
func TestListPartyMembers_OnePerCharacterAndOnlyThisRoom(t *testing.T) {
	s, roomID, first, second, elsewhere := partyFixture(t)

	mustCreateToken(t, s, Token{SceneID: first.ID, Name: "Aria", PartyKey: "aria"})
	mustCreateToken(t, s, Token{SceneID: second.ID, Name: "Aria", PartyKey: "aria"})
	mustCreateToken(t, s, Token{SceneID: first.ID, Name: "Brom", PartyKey: "brom"})
	mustCreateToken(t, s, Token{SceneID: first.ID, Name: "Goblin"})
	mustCreateToken(t, s, Token{SceneID: elsewhere.ID, Name: "Stranger", PartyKey: "aria"})

	members, err := s.ListPartyMembers(roomID)
	if err != nil {
		t.Fatalf("ListPartyMembers: %v", err)
	}
	if got := memberNames(members); !slices.Equal(got, []string{"Aria", "Brom"}) {
		t.Fatalf("members = %v, want [Aria Brom]", got)
	}
}

// The newest version of a character is whichever copy was last made or
// edited, on any scene — so new art given to the copy in front of the GM
// is what the next scene gets, rather than the art from the first night.
func TestListPartyMembers_TakesTheMostRecentlyEditedCopy(t *testing.T) {
	s, roomID, first, second, _ := partyFixture(t)

	original := mustCreateToken(t, s, Token{SceneID: first.ID, Name: "Aria", PartyKey: "aria"})
	mustCreateToken(t, s, Token{SceneID: second.ID, Name: "Aria", PartyKey: "aria"})

	// The older copy is the one edited, which is what makes this a test of
	// updated_at rather than of row order.
	original.Name = "Aria the Bold"
	if err := s.UpdateToken(original); err != nil {
		t.Fatalf("UpdateToken: %v", err)
	}

	members, err := s.ListPartyMembers(roomID)
	if err != nil {
		t.Fatalf("ListPartyMembers: %v", err)
	}
	if len(members) != 1 || members[0].ID != original.ID {
		t.Fatalf("members = %+v, want only the edited copy %s", members, original.ID)
	}
}

// Moving a token isn't an edit to the character, so dragging an old copy
// across its map mustn't make it the version the next scene gets.
func TestListPartyMembers_AMoveDoesNotCountAsAnEdit(t *testing.T) {
	s, roomID, first, second, _ := partyFixture(t)

	old := mustCreateToken(t, s, Token{SceneID: first.ID, Name: "Aria", PartyKey: "aria"})
	newest := mustCreateToken(t, s, Token{SceneID: second.ID, Name: "Aria", PartyKey: "aria"})
	if err := s.MoveToken(old.ID, 4, 4); err != nil {
		t.Fatalf("MoveToken: %v", err)
	}

	members, err := s.ListPartyMembers(roomID)
	if err != nil {
		t.Fatalf("ListPartyMembers: %v", err)
	}
	if len(members) != 1 || members[0].ID != newest.ID {
		t.Fatalf("members = %+v, want the newest copy %s", members, newest.ID)
	}
}

// Leaving is about the character, so every copy on every scene goes —
// and nothing in another room that happens to carry the same key.
func TestLeaveParty_UnkeysEveryCopyInTheRoomAndNothingElse(t *testing.T) {
	s, roomID, first, second, elsewhere := partyFixture(t)

	a := mustCreateToken(t, s, Token{SceneID: first.ID, Name: "Aria", PartyKey: "aria"})
	b := mustCreateToken(t, s, Token{SceneID: second.ID, Name: "Aria", PartyKey: "aria"})
	brom := mustCreateToken(t, s, Token{SceneID: first.ID, Name: "Brom", PartyKey: "brom"})
	stranger := mustCreateToken(t, s, Token{SceneID: elsewhere.ID, Name: "Stranger", PartyKey: "aria"})

	changed, err := s.LeaveParty(roomID, "aria")
	if err != nil {
		t.Fatalf("LeaveParty: %v", err)
	}
	ids := []string{}
	for _, tok := range changed {
		ids = append(ids, tok.ID)
		if tok.PartyKey != "" {
			t.Errorf("returned %s still keyed %q", tok.ID, tok.PartyKey)
		}
	}
	slices.Sort(ids)
	want := []string{a.ID, b.ID}
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Fatalf("changed = %v, want %v", ids, want)
	}

	for _, check := range []struct {
		id, wantKey string
	}{{a.ID, ""}, {b.ID, ""}, {brom.ID, "brom"}, {stranger.ID, "aria"}} {
		got, err := s.GetToken(check.id)
		if err != nil {
			t.Fatalf("GetToken: %v", err)
		}
		if got.PartyKey != check.wantKey {
			t.Errorf("%s: party key %q, want %q", got.Name, got.PartyKey, check.wantKey)
		}
	}
}
