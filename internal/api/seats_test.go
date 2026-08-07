package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The seat-management endpoints are GM-only, so unlike postJSON these
// carry a bearer token.
func postJSONWithToken(t *testing.T, url string, body any, token string) *http.Response {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func deleteWithToken(t *testing.T, url, token string) int {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// Taking a seat is what joining a room means on a device that doesn't
// remember you. These pin the pre-join listing, the claim, and the two
// boundaries around it: the GM seat, and another room's seats.

type seatsBody struct {
	RoomName string         `json:"roomName"`
	Seats    []seatResponse `json:"seats"`
}

func getSeats(t *testing.T, srv *httptest.Server, slug string) seatsBody {
	t.Helper()

	resp, err := http.Get(srv.URL + "/api/rooms/" + slug + "/seats")
	if err != nil {
		t.Fatalf("GET seats: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET seats status = %d, want 200", resp.StatusCode)
	}
	var body seatsBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode seats: %v", err)
	}
	return body
}

func seatNamed(t *testing.T, seats []seatResponse, name string) seatResponse {
	t.Helper()
	for _, seat := range seats {
		if seat.DisplayName == name {
			return seat
		}
	}
	t.Fatalf("no seat named %q in %+v", name, seats)
	return seatResponse{}
}

// The listing has to answer before the caller has a session — it is what
// a device with no session looks at — so it is deliberately reachable
// without one. What it must never grow is anything beyond a name and
// whether the chair is taken.
func TestListSeats_AnswersWithoutASessionAndCarriesNoCredential(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"displayName": "Bob",
	})

	body := getSeats(t, srv, created.RoomSlug)
	if body.RoomName != "Room" {
		t.Errorf("roomName = %q, want Room", body.RoomName)
	}
	if len(body.Seats) != 2 {
		t.Fatalf("len(seats) = %d, want 2 (the GM's and Bob's)", len(body.Seats))
	}

	gm := seatNamed(t, body.Seats, "Alice")
	if gm.Role != "gm" {
		t.Errorf("Alice's role = %q, want gm", gm.Role)
	}
	// Nobody has a socket open in a test server, so nobody is connected —
	// which is the point: a seat with a session is not an occupied chair.
	if gm.Connected {
		t.Error("nobody is connected; occupancy must come from live presence, not the session count")
	}

	// The response is a fixed struct, so this is really a check that
	// nothing was added to it later: seats carry no token.
	raw, err := http.Get(srv.URL + "/api/rooms/" + created.RoomSlug + "/seats")
	if err != nil {
		t.Fatalf("GET seats: %v", err)
	}
	defer raw.Body.Close()
	var loose map[string]any
	if err := json.NewDecoder(raw.Body).Decode(&loose); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, seat := range loose["seats"].([]any) {
		for key := range seat.(map[string]any) {
			switch key {
			case "participantId", "displayName", "role", "connected":
			default:
				t.Errorf("seat carries unexpected field %q — this endpoint is unauthenticated", key)
			}
		}
	}
}

func TestListSeats_UnknownRoomIs404(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/api/rooms/nope/seats")
	if err != nil {
		t.Fatalf("GET seats: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// The story's headline: a device that doesn't remember you takes the
// seat you had, and gets a session of its own on it.
func TestJoin_ClaimingASeatReturnsTheSameParticipant(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var first sessionResponse
	decodeJSONBody(t, postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"displayName": "Bob",
	}), &first)

	// A second device, no stored token, picking Bob off the seat list.
	seat := seatNamed(t, getSeats(t, srv, created.RoomSlug).Seats, "Bob")
	resp := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"participantId": seat.ParticipantID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var second sessionResponse
	decodeJSONBody(t, resp, &second)

	if second.ParticipantID != first.ParticipantID {
		t.Fatalf("claim gave participant %q, want the same seat %q", second.ParticipantID, first.ParticipantID)
	}
	if second.DisplayName != "Bob" {
		t.Errorf("displayName = %q, want Bob", second.DisplayName)
	}
	if second.SessionToken == first.SessionToken {
		t.Error("the second device got the first device's token; each session is its own")
	}

	// And the roster still has one Bob, which is what makes a seat a
	// person rather than a device.
	if n := len(getSeats(t, srv, created.RoomSlug).Seats); n != 2 {
		t.Fatalf("len(seats) = %d, want 2 — claiming must not add a seat", n)
	}
}

// The GM seat is a role boundary: the room password signs you into it,
// so it can't be taken off a list by anyone holding the link.
func TestJoin_ClaimingTheGMSeatIsRefused(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	seat := seatNamed(t, getSeats(t, srv, created.RoomSlug).Seats, "Alice")
	resp := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"participantId": seat.ParticipantID,
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestJoin_ClaimingASeatFromAnotherRoomIs404(t *testing.T) {
	srv, _ := newTestServer(t)
	roomA := createTestRoom(t, srv)

	var other sessionResponse
	decodeJSONBody(t, postJSON(t, srv.URL+"/api/rooms", map[string]string{
		"roomName": "Other", "gmName": "Bea", "password": "hunter2",
	}), &other)
	var bob sessionResponse
	decodeJSONBody(t, postJSON(t, srv.URL+"/api/rooms/"+other.RoomSlug+"/join", map[string]string{
		"displayName": "Bob",
	}), &bob)

	resp := postJSON(t, srv.URL+"/api/rooms/"+roomA.RoomSlug+"/join", map[string]string{
		"participantId": bob.ParticipantID,
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 — a seat is scoped to its room", resp.StatusCode)
	}
}

// A GM logging in from a second device takes the same seat rather than
// growing the roster, which is what it used to do every time.
func TestGMLogin_ReusesTheGMSeat(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	resp := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/gm-login", map[string]string{
		"displayName": "Alice", "password": "hunter2",
	})
	var again sessionResponse
	decodeJSONBody(t, resp, &again)

	if again.ParticipantID != created.ParticipantID {
		t.Fatalf("gm-login gave seat %q, want %q", again.ParticipantID, created.ParticipantID)
	}
	if again.SessionToken == created.SessionToken {
		t.Error("expected a fresh session token for the second device")
	}
	if n := len(getSeats(t, srv, created.RoomSlug).Seats); n != 1 {
		t.Fatalf("len(seats) = %d, want 1 — a second GM login must not add a person", n)
	}
}

func TestSeats_GMAddsAndRemovesThem(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	// Setting the table before anyone arrives.
	resp := postJSONWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/seats",
		map[string]string{"displayName": "Carol"}, created.SessionToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var seat seatResponse
	decodeJSONBody(t, resp, &seat)
	if seat.Role != "player" {
		t.Errorf("added seat role = %q, want player", seat.Role)
	}

	// It's claimable, which is the whole point of adding it early.
	claim := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"participantId": seat.ParticipantID,
	})
	if claim.StatusCode != http.StatusCreated {
		t.Fatalf("claim status = %d, want 201", claim.StatusCode)
	}

	if code := deleteWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/seats/"+seat.ParticipantID,
		created.SessionToken); code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", code)
	}
	if n := len(getSeats(t, srv, created.RoomSlug).Seats); n != 1 {
		t.Fatalf("len(seats) = %d, want 1 after removing Carol's", n)
	}
}

func TestSeats_AddingAndRemovingIsGMOnly(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var bob sessionResponse
	decodeJSONBody(t, postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"displayName": "Bob",
	}), &bob)

	resp := postJSONWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/seats",
		map[string]string{"displayName": "Carol"}, bob.SessionToken)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("player adding a seat: status = %d, want 403", resp.StatusCode)
	}

	if code := deleteWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/seats/"+bob.ParticipantID,
		bob.SessionToken); code != http.StatusForbidden {
		t.Fatalf("player removing a seat: status = %d, want 403", code)
	}

	// And the GM's own seat is refused even to the GM: the room password
	// signs you into it, so removing it would strand the only role that
	// could undo the damage.
	if code := deleteWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/seats/"+created.ParticipantID,
		created.SessionToken); code != http.StatusForbidden {
		t.Fatalf("removing the GM seat: status = %d, want 403", code)
	}
}

// Leaving spends a session, not an identity: the seat survives, and the
// other devices on it stay signed in.
func TestEndSession_SignsOutOneDeviceOnly(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var first sessionResponse
	decodeJSONBody(t, postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"displayName": "Bob",
	}), &first)
	var second sessionResponse
	decodeJSONBody(t, postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"participantId": first.ParticipantID,
	}), &second)

	if code := deleteWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/session",
		first.SessionToken); code != http.StatusNoContent {
		t.Fatalf("leave status = %d, want 204", code)
	}

	if resp := getSession(t, srv, created.RoomSlug, first.SessionToken); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("signed-out token: status = %d, want 401", resp.StatusCode)
	}
	if resp := getSession(t, srv, created.RoomSlug, second.SessionToken); resp.StatusCode != http.StatusOK {
		t.Fatalf("the other device was signed out too: status = %d, want 200", resp.StatusCode)
	}
	// The seat is still there to come back to.
	if n := len(getSeats(t, srv, created.RoomSlug).Seats); n != 2 {
		t.Fatalf("len(seats) = %d, want 2 — leaving must not remove the seat", n)
	}

	// Leaving twice is silence rather than an error: a token that no
	// longer resolves is a device that is already signed out.
	if code := deleteWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/session",
		first.SessionToken); code != http.StatusNoContent {
		t.Fatalf("second leave: status = %d, want 204", code)
	}
}
