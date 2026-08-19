package store

import "testing"

func TestBanner_StartsEmpty(t *testing.T) {
	s := newTestStore(t)

	got, err := s.GetBanner()
	if err != nil {
		t.Fatalf("GetBanner: %v", err)
	}
	if got != "" {
		t.Fatalf("banner = %q, want empty on a fresh database", got)
	}
}

func TestBanner_SetThenRead(t *testing.T) {
	s := newTestStore(t)

	if err := s.SetBanner("Maintenance 19 Aug 8-9pm EST"); err != nil {
		t.Fatalf("SetBanner: %v", err)
	}

	got, err := s.GetBanner()
	if err != nil {
		t.Fatalf("GetBanner: %v", err)
	}
	if got != "Maintenance 19 Aug 8-9pm EST" {
		t.Fatalf("banner = %q, want the message just set", got)
	}
}

// The whole point: a second call has to land on the row the first call
// made, not add a second one — this is what makes SetBanner a plain
// UPDATE safe rather than an upsert.
func TestBanner_SettingTwiceReplacesRatherThanAdding(t *testing.T) {
	s := newTestStore(t)

	if err := s.SetBanner("first"); err != nil {
		t.Fatalf("SetBanner: %v", err)
	}
	if err := s.SetBanner("second"); err != nil {
		t.Fatalf("SetBanner: %v", err)
	}

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM banner`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("banner table has %d rows, want exactly 1", count)
	}

	got, err := s.GetBanner()
	if err != nil {
		t.Fatalf("GetBanner: %v", err)
	}
	if got != "second" {
		t.Fatalf("banner = %q, want the most recent message", got)
	}
}

// The clearing half: an empty string is a real value here, not "unset".
func TestBanner_SetToEmptyClearsIt(t *testing.T) {
	s := newTestStore(t)

	if err := s.SetBanner("back at 9pm"); err != nil {
		t.Fatalf("SetBanner: %v", err)
	}
	if err := s.SetBanner(""); err != nil {
		t.Fatalf("SetBanner(\"\"): %v", err)
	}

	got, err := s.GetBanner()
	if err != nil {
		t.Fatalf("GetBanner: %v", err)
	}
	if got != "" {
		t.Fatalf("banner = %q, want empty after clearing", got)
	}
}

// The row this all depends on is seeded by createTables, not by the
// first SetBanner call — opening the store at all is what a Host's
// `set-banner`/`clear-banner` invocation and the running server both do
// independently, and both have to find the row already there.
func TestBanner_RowExistsAsSoonAsTheStoreOpens(t *testing.T) {
	s := newTestStore(t)

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM banner`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("banner table has %d rows right after New, want exactly 1", count)
	}
}
