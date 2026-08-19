package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"longtable/internal/blobstore"
	"longtable/internal/db"
	"longtable/internal/store"
	"longtable/internal/ws"
)

// The Host's banner. It answers before anyone has a session — it's shown
// on every page, including the home page of a browser that has never
// joined anything — so what it must not do is say anything about the
// server beyond the message the Host typed.

// noticeServer returns a running server and the store behind it, so a
// test can call SetBanner directly the way `longtable set-banner` does
// — against the same database, without restarting anything — rather
// than baking a fixed notice into the router at construction time.
func noticeServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()

	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	s, err := store.New(sqlDB)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	blobs, err := blobstore.New(filepath.Join(t.TempDir(), "blobs"))
	if err != nil {
		t.Fatalf("new blobstore: %v", err)
	}

	srv := httptest.NewServer(NewRouter(s, ws.NewHub(s, ws.DefaultDepartureGrace), blobs, os.DirFS(t.TempDir())))
	t.Cleanup(srv.Close)
	return srv, s
}

func readNotice(t *testing.T, srv *httptest.Server) string {
	t.Helper()

	resp, err := http.Get(srv.URL + "/api/notice")
	if err != nil {
		t.Fatalf("GET /api/notice: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var payload struct {
		Notice string `json:"notice"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return payload.Notice
}

func TestNotice_ServesWhatTheHostSet(t *testing.T) {
	srv, s := noticeServer(t)
	if err := s.SetBanner("Back up at 9pm"); err != nil {
		t.Fatalf("SetBanner: %v", err)
	}

	if got := readNotice(t, srv); got != "Back up at 9pm" {
		t.Fatalf("notice = %q, want the Host's message", got)
	}
}

// An unset banner is the ordinary case, not a miss: the client asks on
// every page load, and a 404 for the normal state would have every Host
// reporting a bug from their browser console.
func TestNotice_AnsweringEmptyIsNotAnError(t *testing.T) {
	srv, _ := noticeServer(t)

	if got := readNotice(t, srv); got != "" {
		t.Fatalf("notice = %q, want empty", got)
	}
}

// The reason getNotice reads the store on every request instead of
// holding a value: `longtable set-banner`/`clear-banner` write to the
// database of a server that is already running, and every later request
// has to see the change without anyone restarting anything. One server,
// never restarted, three different answers.
func TestNotice_ChangesWithoutRestartingTheServer(t *testing.T) {
	srv, s := noticeServer(t)

	if got := readNotice(t, srv); got != "" {
		t.Fatalf("notice = %q, want empty before anything is set", got)
	}

	if err := s.SetBanner("Maintenance 19 Aug 8-9pm EST"); err != nil {
		t.Fatalf("SetBanner: %v", err)
	}
	if got := readNotice(t, srv); got != "Maintenance 19 Aug 8-9pm EST" {
		t.Fatalf("notice = %q, want the message just set", got)
	}

	if err := s.SetBanner(""); err != nil {
		t.Fatalf(`SetBanner(""): %v`, err)
	}
	if got := readNotice(t, srv); got != "" {
		t.Fatalf("notice = %q, want empty after clearing", got)
	}
}
