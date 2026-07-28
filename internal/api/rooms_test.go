package api

import (
	"bytes"
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

// newTestServer wires a real Store (temp SQLite file) and blobstore
// (temp dir) into the full router, so these tests exercise the actual
// HTTP handlers end-to-end. The embedded frontend FS is an empty temp
// dir — fine, since none of these tests touch the SPA fallback route.
func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
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

	hub := ws.NewHub(s)
	frontend := os.DirFS(t.TempDir())
	router := NewRouter(s, hub, blobs, frontend)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv, s
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func decodeJSONBody(t *testing.T, resp *http.Response, v any) {
	t.Helper()

	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// createTestRoom is a shared fixture: creates a room with GM password
// "hunter2" and returns its session response.
func createTestRoom(t *testing.T, srv *httptest.Server) sessionResponse {
	t.Helper()

	resp := postJSON(t, srv.URL+"/api/rooms", map[string]string{
		"roomName": "Room", "gmName": "Alice", "password": "hunter2",
	})
	var session sessionResponse
	decodeJSONBody(t, resp, &session)
	return session
}

func TestCreateRoom_Success(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := postJSON(t, srv.URL+"/api/rooms", map[string]string{
		"roomName": "Curse of Strahd", "gmName": "Alice", "password": "hunter2",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var session sessionResponse
	decodeJSONBody(t, resp, &session)
	if session.Role != "gm" {
		t.Fatalf("role = %q, want gm", session.Role)
	}
	if session.SessionToken == "" {
		t.Fatal("expected a session token")
	}
}

func TestCreateRoom_ValidatesShortPassword(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := postJSON(t, srv.URL+"/api/rooms", map[string]string{
		"roomName": "Room", "gmName": "Alice", "password": "abc",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreateRoom_ValidatesEmptyName(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := postJSON(t, srv.URL+"/api/rooms", map[string]string{
		"roomName": "", "gmName": "Alice", "password": "hunter2",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestJoinRoom_NewPlayer(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	resp := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"displayName": "Bob",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var session sessionResponse
	decodeJSONBody(t, resp, &session)
	if session.Role != "player" {
		t.Fatalf("role = %q, want player", session.Role)
	}
}

func TestJoinRoom_ResumesExistingSession(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	resp := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"sessionToken": created.SessionToken,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var session sessionResponse
	decodeJSONBody(t, resp, &session)
	if session.ParticipantID != created.ParticipantID {
		t.Fatalf("participantId = %q, want %q (should resume the same identity)", session.ParticipantID, created.ParticipantID)
	}
}

func TestJoinRoom_StaleSessionTokenFallsBackToNewPlayer(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	resp := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"sessionToken": "garbage-token", "displayName": "Bob",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (should fall back to creating a new participant)", resp.StatusCode)
	}

	var session sessionResponse
	decodeJSONBody(t, resp, &session)
	if session.Role != "player" {
		t.Fatalf("role = %q, want player", session.Role)
	}
}

func TestJoinRoom_UnknownSlug(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := postJSON(t, srv.URL+"/api/rooms/nosuchroom/join", map[string]string{"displayName": "Bob"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGMLogin_WrongPassword(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	resp := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/gm-login", map[string]string{
		"displayName": "Alice", "password": "wrong",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestGMLogin_CorrectPassword(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	resp := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/gm-login", map[string]string{
		"displayName": "Alice", "password": "hunter2",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var session sessionResponse
	decodeJSONBody(t, resp, &session)
	if session.Role != "gm" {
		t.Fatalf("role = %q, want gm", session.Role)
	}
}

func TestListRooms(t *testing.T) {
	srv, _ := newTestServer(t)
	createTestRoom(t, srv)

	resp, err := http.Get(srv.URL + "/api/rooms")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	var rooms []roomSummary
	decodeJSONBody(t, resp, &rooms)
	if len(rooms) != 1 {
		t.Fatalf("len(rooms) = %d, want 1", len(rooms))
	}
}
