package store

import (
	"slices"
	"testing"
)

// Every list in this file is ordered by a timestamp with rowid behind
// it, and these are the tests for the rowid half.
//
// They force the tie rather than racing for it. `time.Now()` on Windows
// advances in steps of about a millisecond, so rows written in one pass
// share a timestamp there and the tie is the normal case — but on a
// clock fine enough to separate them (Linux CI, say) the timestamps
// alone would order these correctly and the tests would pass whether the
// tie-break existed or not. Flattening the column by hand is what makes
// them mean the same thing on every machine.
//
// The bug they stand against is in
// planning/backlog/rows-ordered-on-a-coarse-clock.md: two of these lists
// were shuffling between reads, one of them decided by a random UUID.
//
// **Only the DESC one has teeth today**, and that is worth knowing
// before trusting the others. Strip every rowid tie-break and rerun:
// TestListRooms fails every time, and the four ascending tests still
// pass, because SQLite's sorter currently returns tied rows in the order
// it scanned them — which is rowid ascending, which is what they assert.
// That is an accident of the query plan rather than a promise: SQLite
// specifies no order for tied rows, and ListRecentMessages (descending,
// same shape) was reordering itself once a run in three. So the four
// ascending cases pin intent rather than catching today's bug — they
// fail if someone sorts by something else, and they are the note saying
// what the order is meant to be.

// flattenTimestamps gives every row in a table the same timestamp, which
// is the state a coarse clock produces on its own.
func flattenTimestamps(t *testing.T, s *Store, table string) {
	t.Helper()

	// Table names can't be bound as parameters, and these are literals
	// from the tests below rather than anything a request supplies.
	if _, err := s.db.Exec(`UPDATE ` + table + ` SET created_at = '2026-08-19T12:00:00Z'`); err != nil {
		t.Fatalf("flatten %s.created_at: %v", table, err)
	}
}

// Later strokes render on top of earlier ones, so a tie here is two
// people's drawings swapping z-order between reloads — and disagreeing
// between their two screens.
func TestListDrawingsForScene_HoldsCreationOrderWhenTimestampsTie(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 10, 10)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	var want []string
	for range 5 {
		d, err := s.CreateDrawing(Drawing{
			SceneID: scene.ID, Kind: DrawingKindLine,
			Points: []Point{{X: 0, Y: 0}, {X: 1, Y: 1}}, Color: "#000000",
		})
		if err != nil {
			t.Fatalf("CreateDrawing: %v", err)
		}
		want = append(want, d.ID)
	}
	flattenTimestamps(t, s, "drawing")

	drawings, err := s.ListDrawingsForScene(scene.ID)
	if err != nil {
		t.Fatalf("ListDrawingsForScene: %v", err)
	}
	got := make([]string, len(drawings))
	for i, d := range drawings {
		got[i] = d.ID
	}
	if !slices.Equal(got, want) {
		t.Fatalf("drawing order = %v, want the order they were drawn in %v", got, want)
	}
}

// The tracker's most ordinary case, not an edge one: a batch of monsters
// added in one go shares an initiative, a sort_order and a timestamp, so
// all three sort keys tie and only rowid is left to decide whose turn
// comes first.
func TestInitiativeOrder_HoldsWhenAWholeBatchTies(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	var want []string
	for _, name := range []string{"Goblin 1", "Goblin 2", "Goblin 3", "Goblin 4"} {
		e, err := s.CreateInitiativeEntry(InitiativeEntry{RoomID: room.ID, Name: name, Initiative: 14})
		if err != nil {
			t.Fatalf("CreateInitiativeEntry: %v", err)
		}
		want = append(want, e.Name)
	}
	flattenTimestamps(t, s, "initiative_entry")

	entries, err := s.ListInitiativeEntries(room.ID)
	if err != nil {
		t.Fatalf("ListInitiativeEntries: %v", err)
	}
	got := make([]string, len(entries))
	for i, e := range entries {
		got[i] = e.Name
	}
	if !slices.Equal(got, []string{"Goblin 1", "Goblin 2", "Goblin 3", "Goblin 4"}) {
		t.Fatalf("turn order = %v, want them in the order they were added %v", got, want)
	}
}

func TestListScenesForRoom_HoldsCreationOrderWhenTimestampsTie(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	want := []string{"Tavern", "Dungeon", "Sewers"}
	for _, name := range want {
		if _, err := s.CreateScene(room.ID, name, nil, 70, 10, 10); err != nil {
			t.Fatalf("CreateScene: %v", err)
		}
	}
	flattenTimestamps(t, s, "scene")

	scenes, err := s.ListScenesForRoom(room.ID)
	if err != nil {
		t.Fatalf("ListScenesForRoom: %v", err)
	}
	got := make([]string, len(scenes))
	for i, sc := range scenes {
		got[i] = sc.Name
	}
	if !slices.Equal(got, want) {
		t.Fatalf("scene order = %v, want the order they were built in %v", got, want)
	}
}

// The roster and the seat list are two queries over one table and are
// read side by side — the rail's badges and the seat picker — so they
// have to agree with each other as well as with themselves.
func TestListParticipants_AndSeats_HoldOrderWhenTimestampsTie(t *testing.T) {
	s := newTestStore(t)

	room, gm, err := s.CreateRoom("Room", "GM", "pw")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	want := []string{gm.DisplayName}
	for _, name := range []string{"Bob", "Carol", "Dan"} {
		if _, err := s.JoinRoom(room.ID, name, ""); err != nil {
			t.Fatalf("JoinRoom: %v", err)
		}
		want = append(want, name)
	}
	flattenTimestamps(t, s, "participant")

	participants, err := s.ListParticipantsForRoom(room.ID)
	if err != nil {
		t.Fatalf("ListParticipantsForRoom: %v", err)
	}
	got := make([]string, len(participants))
	for i, p := range participants {
		got[i] = p.DisplayName
	}
	if !slices.Equal(got, want) {
		t.Fatalf("roster = %v, want the order they arrived in %v", got, want)
	}

	seats, err := s.ListSeatsForRoom(room.ID)
	if err != nil {
		t.Fatalf("ListSeatsForRoom: %v", err)
	}
	gotSeats := make([]string, len(seats))
	for i, seat := range seats {
		gotSeats[i] = seat.DisplayName
	}
	if !slices.Equal(gotSeats, want) {
		t.Fatalf("seats = %v, want the same order the roster is in %v", gotSeats, want)
	}
}

// `longtable room list` is a Host reading a code off a screen. It has to
// print the same order twice running, or they have to check it moved.
func TestListRooms_HoldsNewestFirstWhenTimestampsTie(t *testing.T) {
	s := newTestStore(t)

	var made []string
	for _, name := range []string{"First", "Second", "Third"} {
		room, _, err := s.CreateRoom(name, "GM", "pw")
		if err != nil {
			t.Fatalf("CreateRoom: %v", err)
		}
		made = append(made, room.Name)
	}
	flattenTimestamps(t, s, "room")

	rooms, err := s.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms: %v", err)
	}
	got := make([]string, len(rooms))
	for i, r := range rooms {
		got[i] = r.Name
	}
	want := []string{"Third", "Second", "First"}
	if !slices.Equal(got, want) {
		t.Fatalf("room order = %v, want newest first %v (made %v)", got, want, made)
	}
}
