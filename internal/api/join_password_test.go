package api

import (
	"net/http"
	"testing"
)

// The join password gates joining as a Player, separate from the GM
// password — set over PUT like the GM password (a credential, not a
// command), but unlike it, unset is a valid state rather than something
// to type your way out of.

func TestListSeats_ReportsWhetherAJoinPasswordIsRequired(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	if got := getSeats(t, srv, created.RoomSlug).JoinPasswordRequired; got {
		t.Fatal("joinPasswordRequired = true before any password was set")
	}

	resp := putJSONWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join-password",
		map[string]string{"password": "letmein"}, created.SessionToken)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("set password: status = %d, want 204", resp.StatusCode)
	}

	if got := getSeats(t, srv, created.RoomSlug).JoinPasswordRequired; !got {
		t.Fatal("joinPasswordRequired = false after setting one")
	}
}

func TestSetJoinPassword_IsGMOnlyAndHasAMinimumLength(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var bob sessionResponse
	decodeJSONBody(t, postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"displayName": "Bob",
	}), &bob)

	byPlayer := putJSONWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join-password",
		map[string]string{"password": "player picked this"}, bob.SessionToken)
	byPlayer.Body.Close()
	if byPlayer.StatusCode != http.StatusForbidden {
		t.Errorf("a Player setting it: status = %d, want 403", byPlayer.StatusCode)
	}

	tooShort := putJSONWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join-password",
		map[string]string{"password": "no"}, created.SessionToken)
	tooShort.Body.Close()
	if tooShort.StatusCode != http.StatusBadRequest {
		t.Errorf("a two-character password: status = %d, want 400", tooShort.StatusCode)
	}
}

// Unlike the GM password, an empty value is accepted rather than
// refused: it's how a GM removes the password rather than a typo to
// reject.
func TestSetJoinPassword_EmptyClearsIt(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	set := putJSONWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join-password",
		map[string]string{"password": "letmein"}, created.SessionToken)
	set.Body.Close()

	clear := putJSONWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join-password",
		map[string]string{"password": ""}, created.SessionToken)
	clear.Body.Close()
	if clear.StatusCode != http.StatusNoContent {
		t.Fatalf("clearing: status = %d, want 204", clear.StatusCode)
	}

	if got := getSeats(t, srv, created.RoomSlug).JoinPasswordRequired; got {
		t.Fatal("joinPasswordRequired = true after clearing the password")
	}

	resp := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"displayName": "Bob",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("joining with no password after clearing: status = %d, want 201", resp.StatusCode)
	}
}

// Gates both ways of joining as a Player: taking an existing seat and
// making a new one.
func TestSetJoinPassword_GatesBothWaysOfJoiningAsAPlayer(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var existingSeat sessionResponse
	decodeJSONBody(t, postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"displayName": "Bob",
	}), &existingSeat)

	resp := putJSONWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join-password",
		map[string]string{"password": "letmein"}, created.SessionToken)
	resp.Body.Close()

	newWrong := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"displayName": "Mallory", "joinPassword": "wrong",
	})
	newWrong.Body.Close()
	if newWrong.StatusCode != http.StatusForbidden {
		t.Errorf("new seat, wrong password: status = %d, want 403", newWrong.StatusCode)
	}

	claimWrong := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"participantId": existingSeat.ParticipantID, "joinPassword": "wrong",
	})
	claimWrong.Body.Close()
	if claimWrong.StatusCode != http.StatusForbidden {
		t.Errorf("claiming a seat, wrong password: status = %d, want 403", claimWrong.StatusCode)
	}

	newCorrect := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"displayName": "Mallory", "joinPassword": "letmein",
	})
	newCorrect.Body.Close()
	if newCorrect.StatusCode != http.StatusCreated {
		t.Errorf("new seat, correct password: status = %d, want 201", newCorrect.StatusCode)
	}

	claimCorrect := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"participantId": existingSeat.ParticipantID, "joinPassword": "letmein",
	})
	claimCorrect.Body.Close()
	if claimCorrect.StatusCode != http.StatusCreated {
		t.Errorf("claiming a seat, correct password: status = %d, want 201", claimCorrect.StatusCode)
	}
}

// A device that already has a session resumes without the password: this
// setting gates joining, not being in the room, so a GM adding one
// mid-session doesn't evict anyone already seated.
func TestSetJoinPassword_DoesNotGateResumingSession(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var bob sessionResponse
	decodeJSONBody(t, postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"displayName": "Bob",
	}), &bob)

	resp := putJSONWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join-password",
		map[string]string{"password": "letmein"}, created.SessionToken)
	resp.Body.Close()

	resume := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join", map[string]string{
		"sessionToken": bob.SessionToken,
	})
	if resume.StatusCode != http.StatusOK {
		t.Fatalf("resuming with an existing session: status = %d, want 200", resume.StatusCode)
	}
}

// The GM password is unaffected by this setting, and vice versa: they
// gate different seats.
func TestSetJoinPassword_DoesNotAffectGMLogin(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	resp := putJSONWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join-password",
		map[string]string{"password": "letmein"}, created.SessionToken)
	resp.Body.Close()

	gmLoginResp := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/gm-login", map[string]string{
		"displayName": "Alice", "password": "hunter2",
	})
	gmLoginResp.Body.Close()
	if gmLoginResp.StatusCode != http.StatusCreated {
		t.Fatalf("GM login after setting a join password: status = %d, want 201", gmLoginResp.StatusCode)
	}
}

// checkJoinPassword is what the pre-join screen calls to refuse a wrong
// password immediately, before a Player has picked a seat or typed a
// name — none of which this endpoint touches at all.
func TestCheckJoinPassword(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	// Nothing set yet: anything answers correct, since there's no
	// password for the pre-join screen to have asked about.
	unset := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join-password/check",
		map[string]string{"password": "whatever"})
	unset.Body.Close()
	if unset.StatusCode != http.StatusNoContent {
		t.Errorf("no password set: status = %d, want 204", unset.StatusCode)
	}

	resp := putJSONWithToken(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join-password",
		map[string]string{"password": "letmein"}, created.SessionToken)
	resp.Body.Close()

	wrong := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join-password/check",
		map[string]string{"password": "nope"})
	wrong.Body.Close()
	if wrong.StatusCode != http.StatusForbidden {
		t.Errorf("wrong password: status = %d, want 403", wrong.StatusCode)
	}

	right := postJSON(t, srv.URL+"/api/rooms/"+created.RoomSlug+"/join-password/check",
		map[string]string{"password": "letmein"})
	right.Body.Close()
	if right.StatusCode != http.StatusNoContent {
		t.Errorf("right password: status = %d, want 204", right.StatusCode)
	}
}

// Creating a room is the other place a GM sets this — up front, rather
// than opening Manage room straight afterward.
func TestCreateRoom_WithJoinPassword(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := postJSON(t, srv.URL+"/api/rooms", map[string]string{
		"roomName": "Room", "gmName": "Alice", "password": "hunter2", "joinPassword": "letmein",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var session sessionResponse
	decodeJSONBody(t, resp, &session)

	if got := getSeats(t, srv, session.RoomSlug).JoinPasswordRequired; !got {
		t.Fatal("joinPasswordRequired = false for a room created with one")
	}

	wrong := postJSON(t, srv.URL+"/api/rooms/"+session.RoomSlug+"/join", map[string]string{
		"displayName": "Bob", "joinPassword": "wrong",
	})
	wrong.Body.Close()
	if wrong.StatusCode != http.StatusForbidden {
		t.Errorf("joining with the wrong password: status = %d, want 403", wrong.StatusCode)
	}

	right := postJSON(t, srv.URL+"/api/rooms/"+session.RoomSlug+"/join", map[string]string{
		"displayName": "Bob", "joinPassword": "letmein",
	})
	right.Body.Close()
	if right.StatusCode != http.StatusCreated {
		t.Errorf("joining with the right password: status = %d, want 201", right.StatusCode)
	}
}

// A join password set at creation is held to the same minimum length as
// one set later, so a room can't end up holding one that Manage room
// would have refused.
func TestCreateRoom_ValidatesShortJoinPassword(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := postJSON(t, srv.URL+"/api/rooms", map[string]string{
		"roomName": "Room", "gmName": "Alice", "password": "hunter2", "joinPassword": "no",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}
