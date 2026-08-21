package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"longtable/internal/auth"
	"longtable/internal/store"
)

// sessionResponse is returned by every endpoint that establishes a
// participant session (create room, join, gm-login). The client stores
// sessionToken and uses it both to resume this identity on a later
// visit and to authenticate the WebSocket connection.
type sessionResponse struct {
	RoomSlug      string `json:"roomSlug"`
	RoomName      string `json:"roomName"`
	ParticipantID string `json:"participantId"`
	DisplayName   string `json:"displayName"`
	Color         string `json:"color"`
	Role          string `json:"role"`
	SessionToken  string `json:"sessionToken"`
}

func toSessionResponse(room store.Room, p store.Participant) sessionResponse {
	return sessionResponse{
		RoomSlug:      room.Slug,
		RoomName:      room.Name,
		ParticipantID: p.ID,
		DisplayName:   p.DisplayName,
		Color:         p.Color,
		Role:          string(p.Role),
		SessionToken:  p.SessionToken,
	}
}

// rejectUnknownColor guards every path that writes a colour. The set
// lives in Go rather than in a CHECK constraint (see the note above
// addMissingColumns), and it has to be enforced somewhere: the stored
// value ends up in a style attribute on every other client's screen, so
// "the table is trusted" (ADR-0007) does not stretch to letting a
// crafted request put arbitrary text there.
func rejectUnknownColor(w http.ResponseWriter, color string) bool {
	if store.ValidIdentityColor(color) {
		return false
	}
	writeError(w, http.StatusBadRequest, "unknown colour")
	return true
}

// No colour, here or on gmLoginRequest. A GM seat is black, decided by
// the role when it's drawn rather than stored — so there is nothing for a
// client to send, and an empty key is what the seat keeps.
type createRoomRequest struct {
	RoomName string `json:"roomName"`
	GMName   string `json:"gmName"`
	Password string `json:"password"`
	// Optional, and separate from Password above: this one gates joining
	// as a Player rather than the GM seat. Empty leaves it unset, same as
	// a room that never touches this at all.
	JoinPassword string `json:"joinPassword"`
}

// minGMPasswordLength is the whole of the password policy, and it is
// this short on purpose: the room password is said out loud across a
// table, not typed by a stranger over the internet. It guards against a
// slip of the keyboard rather than against an attacker.
//
// Enforced everywhere a password is set, so a room can't end up holding
// one that the form which created it would have refused.
const minGMPasswordLength = 4

func rejectShortPassword(w http.ResponseWriter, password string) bool {
	if len(password) < minGMPasswordLength {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("password must be at least %d characters", minGMPasswordLength))
		return true
	}
	return false
}

func (srv *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	var req createRoomRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.RoomName = strings.TrimSpace(req.RoomName)
	req.GMName = strings.TrimSpace(req.GMName)

	if req.RoomName == "" {
		writeError(w, http.StatusBadRequest, "roomName is required")
		return
	}
	if req.GMName == "" {
		writeError(w, http.StatusBadRequest, "gmName is required")
		return
	}
	if rejectShortPassword(w, req.Password) {
		return
	}
	// Checked before creating anything, so a room never exists half
	// configured — a join password too short to have been accepted on
	// its own is refused here rather than silently dropped.
	if req.JoinPassword != "" && rejectShortPassword(w, req.JoinPassword) {
		return
	}

	room, participant, err := srv.store.CreateRoom(req.RoomName, req.GMName, req.Password)
	if err != nil {
		slog.Error("api: create room failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create room")
		return
	}

	if req.JoinPassword != "" {
		if err := srv.store.SetJoinPassword(room.ID, req.JoinPassword); err != nil {
			slog.Error("api: set join password on create failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to create room")
			return
		}
	}

	writeJSON(w, http.StatusCreated, toSessionResponse(room, participant))
}

// seatResponse is a room's seat as the pre-join screen sees it.
//
// Deliberately narrow: this endpoint is reachable by anyone holding the
// room's link and before they have proved anything about who they are.
// A name and whether the chair is occupied is what a seat picker needs;
// anything more would be telling a stranger about the table.
type seatResponse struct {
	ParticipantID string `json:"participantId"`
	DisplayName   string `json:"displayName"`
	// The seat's colour, so the picker can show which are taken before
	// anyone chooses one. A name and a colour is more than this endpoint
	// used to tell a stranger with the link and still less than a
	// credential — and without it the "see what everyone else picked"
	// half of the feature can't happen before joining, which is the only
	// moment it is any use.
	Color     string `json:"color"`
	Role      string `json:"role"`
	Connected bool   `json:"connected"`
}

// listSeats answers "which chairs are at this table, and is anyone in
// them" for a device with no session yet.
//
// Deliberately unauthenticated, because it has to be: it is what you
// look at *before* you have a session. That is not a room-enumeration
// hole — there is no endpoint listing rooms, so reaching this means
// already holding the link — but it is the reason the payload stays as
// thin as it is. See ADR-0008 and ADR-0007.
func (srv *Server) listSeats(w http.ResponseWriter, r *http.Request) {
	room, ok := srv.lookupRoom(w, r.PathValue("slug"))
	if !ok {
		return
	}

	seats, err := srv.store.ListSeatsForRoom(room.ID)
	if err != nil {
		slog.Error("api: list seats failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list seats")
		return
	}

	// "Occupied" is a live question, so it comes from the hub rather than
	// from the session count: a seat with two sessions and nobody
	// connected is a returning player's, not a taken chair.
	connected := make(map[string]bool)
	for _, id := range srv.hub.ConnectedParticipantIDs(room.ID) {
		connected[id] = true
	}

	out := make([]seatResponse, 0, len(seats))
	for _, seat := range seats {
		out = append(out, seatResponse{
			ParticipantID: seat.ID,
			DisplayName:   seat.DisplayName,
			Color:         seat.Color,
			Role:          string(seat.Role),
			Connected:     connected[seat.ID],
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"roomName": room.Name,
		"seats":    out,
		// Whether the picker needs to ask for a password before joining —
		// never the password itself. Already the kind of thing this
		// endpoint tells a stranger with the link (see the type's own
		// comment), not a new exposure.
		"joinPasswordRequired": room.JoinPasswordHash != "",
	})
}

// acceptsJoinPassword reports whether password is good enough to join
// room as a Player — trivially true when the room has none set, so a
// caller never has to branch on that itself.
func acceptsJoinPassword(room store.Room, password string) bool {
	return room.JoinPasswordHash == "" || auth.VerifyPassword(room.JoinPasswordHash, password)
}

type checkJoinPasswordRequest struct {
	Password string `json:"password"`
}

// checkJoinPassword answers "is this right?" without joining anything,
// so the pre-join screen can refuse a wrong password the moment it's
// typed rather than after a Player has also picked a seat, a colour and
// a name — all of which `joinRoom` would otherwise discard on a refusal.
// Unauthenticated like listSeats, for the same reason: it has to answer
// before there's a session to authenticate.
func (srv *Server) checkJoinPassword(w http.ResponseWriter, r *http.Request) {
	room, ok := srv.lookupRoom(w, r.PathValue("slug"))
	if !ok {
		return
	}

	var req checkJoinPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !acceptsJoinPassword(room, req.Password) {
		writeError(w, http.StatusForbidden, "incorrect room password")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type joinRequest struct {
	DisplayName  string `json:"displayName"`
	Color        string `json:"color"`
	SessionToken string `json:"sessionToken"`
	// Set to take an existing seat rather than make a new one. Empty is
	// the "I'm new here" path, which is exactly what joining used to be.
	ParticipantID string `json:"participantId"`
	// Checked against the room's join password when one is set. Ignored
	// otherwise, and never asked of a resuming sessionToken — that device
	// already proved its seat the last time it joined.
	JoinPassword string `json:"joinPassword"`
}

func (srv *Server) joinRoom(w http.ResponseWriter, r *http.Request) {
	room, ok := srv.lookupRoom(w, r.PathValue("slug"))
	if !ok {
		return
	}

	var req joinRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// A returning browser that still has its token resumes without
	// touching the seat picker at all.
	if req.SessionToken != "" {
		if p, err := srv.store.GetParticipantByToken(room.ID, req.SessionToken); err == nil {
			writeJSON(w, http.StatusOK, toSessionResponse(room, p))
			return
		}
	}

	// Gates both branches below — taking a seat and making one — but not
	// the resume above: a device with a sessionToken already proved its
	// seat, and this password gates joining, not being in the room. A GM
	// adding or changing it doesn't evict anyone already seated.
	if !acceptsJoinPassword(room, req.JoinPassword) {
		writeError(w, http.StatusForbidden, "incorrect room password")
		return
	}

	// Taking a seat someone already sat in. No secret and no approval —
	// seats are open-claim, bounded by needing the room's link to get
	// here (ADR-0007). The GM's seat is the exception.
	if req.ParticipantID != "" {
		participant, err := srv.store.ClaimSeat(room.ID, req.ParticipantID)
		switch {
		case errors.Is(err, store.ErrGMSeatNeedsPassword):
			writeError(w, http.StatusForbidden, "the GM seat needs the room password")
			return
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "seat not found")
			return
		case err != nil:
			slog.Error("api: claim seat failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to take that seat")
			return
		}
		writeJSON(w, http.StatusCreated, toSessionResponse(room, participant))
		return
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "displayName is required")
		return
	}

	if rejectUnknownColor(w, req.Color) {
		return
	}

	participant, err := srv.store.JoinRoom(room.ID, req.DisplayName, req.Color)
	if err != nil {
		slog.Error("api: join room failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to join room")
		return
	}

	writeJSON(w, http.StatusCreated, toSessionResponse(room, participant))
}

type gmLoginRequest struct {
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
}

func (srv *Server) gmLogin(w http.ResponseWriter, r *http.Request) {
	room, ok := srv.lookupRoom(w, r.PathValue("slug"))
	if !ok {
		return
	}

	var req gmLoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !auth.VerifyPassword(room.GMPasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "incorrect password")
		return
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "displayName is required")
		return
	}

	participant, err := srv.store.GMLogin(room.ID, req.DisplayName)
	if err != nil {
		slog.Error("api: gm login failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to log in")
		return
	}

	writeJSON(w, http.StatusCreated, toSessionResponse(room, participant))
}

type createSeatRequest struct {
	DisplayName string `json:"displayName"`
	Color       string `json:"color"`
}

// createSeat lets a GM set the table before anyone arrives: a named
// chair with nobody signed into it, waiting to be claimed.
func (srv *Server) createSeat(w http.ResponseWriter, r *http.Request) {
	room, ok := srv.lookupRoom(w, r.PathValue("slug"))
	if !ok {
		return
	}
	if !srv.requireGM(w, r, room) {
		return
	}

	var req createSeatRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "displayName is required")
		return
	}

	if rejectUnknownColor(w, req.Color) {
		return
	}

	seat, err := srv.store.CreateSeat(room.ID, req.DisplayName, req.Color)
	if err != nil {
		slog.Error("api: create seat failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to add a seat")
		return
	}

	writeJSON(w, http.StatusCreated, seatResponse{
		ParticipantID: seat.ID,
		DisplayName:   seat.DisplayName,
		Color:         seat.Color,
		Role:          string(seat.Role),
	})
}

// deleteSeat removes a seat and every device signed into it. Seats
// accumulate over a campaign, so a GM needs to be able to clear one out.
func (srv *Server) deleteSeat(w http.ResponseWriter, r *http.Request) {
	room, ok := srv.lookupRoom(w, r.PathValue("slug"))
	if !ok {
		return
	}
	if !srv.requireGM(w, r, room) {
		return
	}

	err := srv.store.DeleteSeat(room.ID, r.PathValue("id"))
	switch {
	case errors.Is(err, store.ErrCannotDeleteGMSeat):
		writeError(w, http.StatusForbidden, "the GM seat can't be removed")
		return
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "seat not found")
		return
	case err != nil:
		slog.Error("api: delete seat failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove that seat")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// deleteRoom removes a room and everything in it, for a GM who is done
// with it. There is no undo, which is why the button that calls this
// asks first — see manage-room-dialog.svelte.
//
// The row goes before anyone is told, so nothing can join or reconnect
// into the gap; the hub then tells whoever is still connected and closes
// their sockets. The other order would leave a window where the room
// says it is gone and still answers.
func (srv *Server) deleteRoom(w http.ResponseWriter, r *http.Request) {
	room, ok := srv.lookupRoom(w, r.PathValue("slug"))
	if !ok {
		return
	}
	if !srv.requireGM(w, r, room) {
		return
	}

	if err := srv.store.DeleteRoom(room.ID); err != nil {
		slog.Error("api: delete room failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete the room")
		return
	}
	srv.hub.RoomDeleted(r.Context(), room.ID)

	w.WriteHeader(http.StatusNoContent)
}

type setGMPasswordRequest struct {
	Password string `json:"password"`
}

// setGMPassword changes the password that signs somebody into this
// room's GM seat, for a GM who is already in it.
//
// The current password is deliberately not asked for. The session token
// proves the seat, which is what every other Manage room action goes on
// (ADR-0007 draws the line at role boundaries, not identity ones), and
// re-asking wouldn't help the case that actually happens: a GM who has
// lost the password can't type it either, and that path runs through the
// Host's `longtable room reset-password`.
//
// Existing sessions are untouched — nothing here reads or writes the
// session table, so nobody is signed out by their password changing
// under them, including whoever just changed it.
func (srv *Server) setGMPassword(w http.ResponseWriter, r *http.Request) {
	room, ok := srv.lookupRoom(w, r.PathValue("slug"))
	if !ok {
		return
	}
	if !srv.requireGM(w, r, room) {
		return
	}

	var req setGMPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if rejectShortPassword(w, req.Password) {
		return
	}

	if err := srv.store.SetGMPassword(room.ID, req.Password); err != nil {
		slog.Error("api: set gm password failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to change the password")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type setJoinPasswordRequest struct {
	Password string `json:"password"`
}

// setJoinPassword sets, changes or clears the password a Player needs to
// join this room. GM-only, and independent of the GM password: this one
// gates joining rather than the GM seat.
//
// Empty is accepted rather than rejected by rejectShortPassword — unlike
// the GM password, which always exists, "no password" is this setting's
// own valid state, and it's how a GM removes one. The minimum length only
// applies to an actual password being set.
//
// Broadcasts afterward because, unlike the GM password, whether one is
// set is visible to the room (never the password itself) — the pre-join
// seat list already has to say so — so a GM's own other tab watching
// Manage room should see it flip without a reload.
func (srv *Server) setJoinPassword(w http.ResponseWriter, r *http.Request) {
	room, ok := srv.lookupRoom(w, r.PathValue("slug"))
	if !ok {
		return
	}
	if !srv.requireGM(w, r, room) {
		return
	}

	var req setJoinPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Password != "" && rejectShortPassword(w, req.Password) {
		return
	}

	if err := srv.store.SetJoinPassword(room.ID, req.Password); err != nil {
		slog.Error("api: set join password failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to change the password")
		return
	}

	room, err := srv.store.GetRoomByID(room.ID)
	if err != nil {
		slog.Error("api: lookup room after join password change failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to change the password")
		return
	}
	srv.hub.BroadcastRoomUpdated(r.Context(), room)

	w.WriteHeader(http.StatusNoContent)
}

// requireGM is requireParticipant plus the role check, for the endpoints
// that manage the room rather than play in it.
func (srv *Server) requireGM(w http.ResponseWriter, r *http.Request, room store.Room) bool {
	token := bearerToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing session token")
		return false
	}
	participant, err := srv.store.GetParticipantByToken(room.ID, token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid session")
		return false
	}
	if participant.Role != store.RoleGM {
		writeError(w, http.StatusForbidden, "that action is GM-only")
		return false
	}
	return true
}

func (srv *Server) lookupRoom(w http.ResponseWriter, slug string) (store.Room, bool) {
	room, err := srv.store.GetRoomBySlug(slug)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "room not found")
		return store.Room{}, false
	}
	if err != nil {
		slog.Error("api: lookup room failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to look up room")
		return store.Room{}, false
	}
	return room, true
}
