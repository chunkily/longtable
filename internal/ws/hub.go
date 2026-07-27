// Package ws is the real-time sync layer: the Go server is the
// authoritative source of truth for a room's state. Clients send
// commands (intent); the hub validates and applies them through the
// store, then broadcasts the resulting event to every client in that
// room.
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/coder/websocket"

	"longtable/internal/dice"
	"longtable/internal/store"
)

type client struct {
	conn        *websocket.Conn
	roomID      string
	participant store.Participant
}

type envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Hub struct {
	store *store.Store

	mu    sync.Mutex
	rooms map[string]map[*client]struct{}
}

func NewHub(s *store.Store) *Hub {
	return &Hub{store: s, rooms: make(map[string]map[*client]struct{})}
}

// ServeHTTP upgrades the request to a WebSocket connection. The room
// slug and a participant session token are required as query
// parameters — there is no way to reach a room's live state without a
// valid session, mirroring the REST endpoints' auth.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("room")
	token := r.URL.Query().Get("token")
	if slug == "" || token == "" {
		http.Error(w, "room and token query parameters are required", http.StatusBadRequest)
		return
	}

	room, err := h.store.GetRoomBySlug(slug)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		} else {
			slog.Error("ws: lookup room failed", "error", err)
		}
		http.Error(w, "room not found", status)
		return
	}

	participant, err := h.store.GetParticipantByToken(room.ID, token)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusUnauthorized
		} else {
			slog.Error("ws: lookup participant failed", "error", err)
		}
		http.Error(w, "invalid session", status)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		slog.Warn("ws: accept failed", "error", err)
		return
	}
	defer conn.CloseNow()

	c := &client{conn: conn, roomID: room.ID, participant: participant}
	h.register(c)
	defer h.unregister(c)

	ctx := r.Context()
	h.sendStateSync(ctx, c, room)

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return // client disconnected or context canceled
		}
		h.handleMessage(ctx, c, data)
	}
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[c.roomID] == nil {
		h.rooms[c.roomID] = make(map[*client]struct{})
	}
	h.rooms[c.roomID][c] = struct{}{}
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms[c.roomID], c)
	if len(h.rooms[c.roomID]) == 0 {
		delete(h.rooms, c.roomID)
	}
}

// broadcast sends an event to every client connected to roomID.
func (h *Hub) broadcast(ctx context.Context, roomID, eventType string, payload any) {
	data, err := marshalEnvelope(eventType, payload)
	if err != nil {
		slog.Error("ws: marshal broadcast failed", "error", err)
		return
	}

	h.mu.Lock()
	clients := make([]*client, 0, len(h.rooms[roomID]))
	for c := range h.rooms[roomID] {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	for _, c := range clients {
		if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
			slog.Warn("ws: broadcast write failed", "error", err)
		}
	}
}

// send delivers an event to a single client (used for state.sync and
// per-client error responses).
func (h *Hub) send(ctx context.Context, c *client, eventType string, payload any) {
	data, err := marshalEnvelope(eventType, payload)
	if err != nil {
		slog.Error("ws: marshal message failed", "error", err)
		return
	}
	if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
		slog.Warn("ws: write failed", "error", err)
	}
}

func (h *Hub) sendError(ctx context.Context, c *client, message string) {
	h.send(ctx, c, "error", map[string]string{"message": message})
}

func marshalEnvelope(eventType string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{Type: eventType, Payload: body})
}

func (h *Hub) handleMessage(ctx context.Context, c *client, data []byte) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		h.sendError(ctx, c, "malformed message")
		return
	}

	switch env.Type {
	case "token.move":
		h.handleTokenMove(ctx, c, env.Payload)
	case "token.create":
		h.handleTokenCreate(ctx, c, env.Payload)
	case "fog.reveal":
		h.handleFogReveal(ctx, c, env.Payload)
	case "scene.create":
		h.handleSceneCreate(ctx, c, env.Payload)
	case "scene.setActive":
		h.handleSceneSetActive(ctx, c, env.Payload)
	case "roll.request":
		h.handleRollRequest(ctx, c, env.Payload)
	default:
		h.sendError(ctx, c, fmt.Sprintf("unknown command type %q", env.Type))
	}
}

func (h *Hub) requireGM(ctx context.Context, c *client) bool {
	if c.participant.Role != store.RoleGM {
		h.sendError(ctx, c, "that action is GM-only")
		return false
	}
	return true
}

// requireSceneInRoom confirms sceneID belongs to c's room before a
// command is allowed to touch it.
func (h *Hub) requireSceneInRoom(ctx context.Context, c *client, sceneID string) bool {
	roomID, err := h.store.SceneRoomID(sceneID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("ws: lookup scene room failed", "error", err)
		}
		h.sendError(ctx, c, "scene not found")
		return false
	}
	if roomID != c.roomID {
		h.sendError(ctx, c, "scene not found")
		return false
	}
	return true
}

// requireAssetExists confirms a referenced asset ID actually resolves
// to an uploaded asset, so scenes/tokens never point at a dangling ID.
// A nil assetID (no image set) is always allowed.
func (h *Hub) requireAssetExists(ctx context.Context, c *client, assetID *string) bool {
	if assetID == nil {
		return true
	}
	if _, err := h.store.GetAsset(*assetID); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("ws: lookup asset failed", "error", err)
		}
		h.sendError(ctx, c, "asset not found")
		return false
	}
	return true
}

func decodePayload(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return fmt.Errorf("missing payload")
	}
	return json.Unmarshal(raw, v)
}

type tokenMoveRequest struct {
	TokenID string  `json:"tokenId"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
}

func (h *Hub) handleTokenMove(ctx context.Context, c *client, raw json.RawMessage) {
	var req tokenMoveRequest
	if err := decodePayload(raw, &req); err != nil || req.TokenID == "" {
		h.sendError(ctx, c, "invalid token.move payload")
		return
	}

	roomID, err := h.store.TokenRoomID(req.TokenID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("ws: lookup token room failed", "error", err)
		}
		h.sendError(ctx, c, "token not found")
		return
	}
	if roomID != c.roomID {
		h.sendError(ctx, c, "token not found")
		return
	}

	if err := h.store.MoveToken(req.TokenID, req.X, req.Y); err != nil {
		slog.Error("ws: move token failed", "error", err)
		h.sendError(ctx, c, "failed to move token")
		return
	}

	h.broadcast(ctx, c.roomID, "token.moved", map[string]any{
		"tokenId": req.TokenID,
		"x":       req.X,
		"y":       req.Y,
	})
}

type tokenCreateRequest struct {
	SceneID            string  `json:"sceneId"`
	Name               string  `json:"name"`
	ImageAssetID       *string `json:"imageAssetId"`
	X                  float64 `json:"x"`
	Y                  float64 `json:"y"`
	Width              float64 `json:"width"`
	Height             float64 `json:"height"`
	OwnerParticipantID *string `json:"ownerParticipantId"`
	Visibility         string  `json:"visibility"`
}

func (h *Hub) handleTokenCreate(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req tokenCreateRequest
	if err := decodePayload(raw, &req); err != nil || req.SceneID == "" || req.Name == "" {
		h.sendError(ctx, c, "invalid token.create payload")
		return
	}
	if !h.requireSceneInRoom(ctx, c, req.SceneID) {
		return
	}
	if !h.requireAssetExists(ctx, c, req.ImageAssetID) {
		return
	}

	visibility := store.Visibility(req.Visibility)
	if visibility == "" {
		visibility = store.VisibilityVisible
	}
	if visibility != store.VisibilityVisible && visibility != store.VisibilityHidden {
		h.sendError(ctx, c, "visibility must be \"visible\" or \"hidden\"")
		return
	}

	width, height := req.Width, req.Height
	if width == 0 {
		width = 1
	}
	if height == 0 {
		height = 1
	}

	token, err := h.store.CreateToken(store.Token{
		SceneID:            req.SceneID,
		Name:               req.Name,
		ImageAssetID:       req.ImageAssetID,
		X:                  req.X,
		Y:                  req.Y,
		Width:              width,
		Height:             height,
		OwnerParticipantID: req.OwnerParticipantID,
		Visibility:         visibility,
	})
	if err != nil {
		slog.Error("ws: create token failed", "error", err)
		h.sendError(ctx, c, "failed to create token")
		return
	}

	h.broadcast(ctx, c.roomID, "token.created", tokenPayload(token))
}

type fogRevealRequest struct {
	SceneID string          `json:"sceneId"`
	Cells   []store.FogCell `json:"cells"`
}

func (h *Hub) handleFogReveal(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req fogRevealRequest
	if err := decodePayload(raw, &req); err != nil || req.SceneID == "" || len(req.Cells) == 0 {
		h.sendError(ctx, c, "invalid fog.reveal payload")
		return
	}
	if !h.requireSceneInRoom(ctx, c, req.SceneID) {
		return
	}

	if err := h.store.RevealCells(req.SceneID, req.Cells); err != nil {
		slog.Error("ws: reveal fog failed", "error", err)
		h.sendError(ctx, c, "failed to reveal fog")
		return
	}

	h.broadcast(ctx, c.roomID, "fog.revealed", map[string]any{
		"sceneId": req.SceneID,
		"cells":   req.Cells,
	})
}

type sceneCreateRequest struct {
	Name       string  `json:"name"`
	MapAssetID *string `json:"mapAssetId"`
	GridSize   int     `json:"gridSize"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
}

func (h *Hub) handleSceneCreate(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req sceneCreateRequest
	if err := decodePayload(raw, &req); err != nil || req.Name == "" {
		h.sendError(ctx, c, "invalid scene.create payload")
		return
	}
	if !h.requireAssetExists(ctx, c, req.MapAssetID) {
		return
	}
	if req.GridSize == 0 {
		req.GridSize = 70
	}

	scene, err := h.store.CreateScene(c.roomID, req.Name, req.MapAssetID, req.GridSize, req.Width, req.Height)
	if err != nil {
		slog.Error("ws: create scene failed", "error", err)
		h.sendError(ctx, c, "failed to create scene")
		return
	}

	h.broadcast(ctx, c.roomID, "scene.created", scenePayload(scene))
}

type sceneSetActiveRequest struct {
	SceneID string `json:"sceneId"`
}

func (h *Hub) handleSceneSetActive(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req sceneSetActiveRequest
	if err := decodePayload(raw, &req); err != nil || req.SceneID == "" {
		h.sendError(ctx, c, "invalid scene.setActive payload")
		return
	}
	if !h.requireSceneInRoom(ctx, c, req.SceneID) {
		return
	}

	if err := h.store.SetActiveScene(c.roomID, req.SceneID); err != nil {
		slog.Error("ws: set active scene failed", "error", err)
		h.sendError(ctx, c, "failed to activate scene")
		return
	}

	h.broadcast(ctx, c.roomID, "scene.activated", map[string]string{"sceneId": req.SceneID})
}

type rollRequest struct {
	Expression string `json:"expression"`
}

func (h *Hub) handleRollRequest(ctx context.Context, c *client, raw json.RawMessage) {
	var req rollRequest
	if err := decodePayload(raw, &req); err != nil || req.Expression == "" {
		h.sendError(ctx, c, "invalid roll.request payload")
		return
	}

	result, err := dice.Roll(req.Expression)
	if err != nil {
		h.sendError(ctx, c, fmt.Sprintf("invalid dice expression: %v", err))
		return
	}

	participantID := c.participant.ID
	roll, err := h.store.InsertRoll(store.Roll{
		RoomID:          c.roomID,
		ParticipantID:   &participantID,
		ParticipantName: c.participant.DisplayName,
		Expression:      result.Expression,
		Result:          result.Total,
		Breakdown:       result.Breakdown,
	})
	if err != nil {
		slog.Error("ws: insert roll failed", "error", err)
		h.sendError(ctx, c, "failed to record roll")
		return
	}

	h.broadcast(ctx, c.roomID, "roll.result", rollPayload(roll))
}
