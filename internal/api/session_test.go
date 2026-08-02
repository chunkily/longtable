package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// GET /api/rooms/{slug}/session exists for the client's reconnect loop,
// which needs to tell "the server is restarting, keep trying" from "this
// session is gone, stop and rejoin". A refused WebSocket upgrade can't
// make that distinction, so these pin the three answers it gives.

func getSession(t *testing.T, srv *httptest.Server, slug, token string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/rooms/"+slug+"/session", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET session: %v", err)
	}
	return resp
}

func TestCheckSession_AnswersForAValidSession(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	resp := getSession(t, srv, created.RoomSlug, created.SessionToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		ParticipantID string `json:"participantId"`
		DisplayName   string `json:"displayName"`
		Role          string `json:"role"`
		SessionToken  string `json:"sessionToken"`
	}
	decodeJSONBody(t, resp, &body)

	if body.ParticipantID != created.ParticipantID {
		t.Fatalf("participantId = %q, want %q", body.ParticipantID, created.ParticipantID)
	}
	if body.Role != "gm" {
		t.Fatalf("role = %q, want gm", body.Role)
	}
	// The caller already holds the token; repeating it in a response is
	// one redirect or one log line away from leaking a credential.
	if body.SessionToken != "" {
		t.Fatal("the response echoed the session token back")
	}
}

// 401 and 404 are the two "stop retrying" answers, and the client treats
// them alike — but they have to be distinguishable from a server that is
// merely having a bad time, which is anything else.
func TestCheckSession_RefusesABadTokenAndAMissingRoom(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	if resp := getSession(t, srv, created.RoomSlug, ""); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token: status = %d, want 401", resp.StatusCode)
	}
	if resp := getSession(t, srv, created.RoomSlug, "garbage"); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad token: status = %d, want 401", resp.StatusCode)
	}
	if resp := getSession(t, srv, "nosuchroom", created.SessionToken); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing room: status = %d, want 404", resp.StatusCode)
	}
}

// A token is only good for the room it was issued for — the same rule
// the WebSocket upgrade applies, and the reason this probe can stand in
// for it at all.
func TestCheckSession_RefusesATokenFromAnotherRoom(t *testing.T) {
	srv, _ := newTestServer(t)
	roomA := createTestRoom(t, srv)

	respB := postJSON(t, srv.URL+"/api/rooms", map[string]string{
		"roomName": "Other", "gmName": "Bob", "password": "hunter2",
	})
	var roomB sessionResponse
	decodeJSONBody(t, respB, &roomB)

	resp := getSession(t, srv, roomB.RoomSlug, roomA.SessionToken)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for a token from another room", resp.StatusCode)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if strings.Contains(string(body), roomA.SessionToken) {
		t.Fatal("the refusal echoed the token back")
	}
}
