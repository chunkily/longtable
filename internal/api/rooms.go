package api

import (
	"errors"
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
	Role          string `json:"role"`
	SessionToken  string `json:"sessionToken"`
}

func toSessionResponse(room store.Room, p store.Participant) sessionResponse {
	return sessionResponse{
		RoomSlug:      room.Slug,
		RoomName:      room.Name,
		ParticipantID: p.ID,
		DisplayName:   p.DisplayName,
		Role:          string(p.Role),
		SessionToken:  p.SessionToken,
	}
}

type createRoomRequest struct {
	RoomName string `json:"roomName"`
	GMName   string `json:"gmName"`
	Password string `json:"password"`
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
	if len(req.Password) < 4 {
		writeError(w, http.StatusBadRequest, "password must be at least 4 characters")
		return
	}

	room, participant, err := srv.store.CreateRoom(req.RoomName, req.GMName, req.Password)
	if err != nil {
		slog.Error("api: create room failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create room")
		return
	}

	writeJSON(w, http.StatusCreated, toSessionResponse(room, participant))
}

type joinRequest struct {
	DisplayName  string `json:"displayName"`
	SessionToken string `json:"sessionToken"`
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

	// A returning browser can resume its existing identity instead of
	// joining as a brand new participant.
	if req.SessionToken != "" {
		if p, err := srv.store.GetParticipantByToken(room.ID, req.SessionToken); err == nil {
			writeJSON(w, http.StatusOK, toSessionResponse(room, p))
			return
		}
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "displayName is required")
		return
	}

	participant, err := srv.store.JoinRoom(room.ID, req.DisplayName)
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
