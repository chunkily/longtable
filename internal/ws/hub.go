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
	"strings"
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

// broadcastPerClient sends an event to every client in roomID, but lets
// build compute a different payload per recipient — used wherever a
// hidden token might be in the payload, since only the GM should
// receive those. build returning nil skips that recipient entirely.
func (h *Hub) broadcastPerClient(ctx context.Context, roomID, eventType string, build func(c *client) any) {
	h.mu.Lock()
	clients := make([]*client, 0, len(h.rooms[roomID]))
	for c := range h.rooms[roomID] {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	for _, c := range clients {
		if payload := build(c); payload != nil {
			h.send(ctx, c, eventType, payload)
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
	case "chat.send":
		h.handleChatSend(ctx, c, env.Payload)
	case "draw.create":
		h.handleDrawCreate(ctx, c, env.Payload)
	case "draw.delete":
		h.handleDrawDelete(ctx, c, env.Payload)
	case "ping":
		h.handlePing(ctx, c, env.Payload)
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

	payload := tokenPayload(token)
	h.broadcastPerClient(ctx, c.roomID, "token.created", func(recipient *client) any {
		if token.Visibility == store.VisibilityHidden && recipient.participant.Role != store.RoleGM {
			return nil
		}
		return payload
	})
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

// drawingKinds maps the wire-format kind string to its typed
// store.DrawingKind, and doubles as the set of recognized kinds.
var drawingKinds = map[string]store.DrawingKind{
	"freehand": store.DrawingKindFreehand,
	"line":     store.DrawingKindLine,
	"rect":     store.DrawingKindRect,
	"ellipse":  store.DrawingKindEllipse,
}

// defaultDrawingColor matches the frontend's color palette (see
// game-canvas.svelte) — used only if a client omits color entirely.
const defaultDrawingColor = "#cc0000"

type drawCreateRequest struct {
	SceneID string        `json:"sceneId"`
	Kind    string        `json:"kind"`
	Points  []store.Point `json:"points"`
	Color   string        `json:"color"`
}

// handleDrawCreate persists a map annotation. Unlike token/scene
// commands, this isn't GM-only — drawing and pinging are meant as a
// shared communication tool for everyone at the table.
func (h *Hub) handleDrawCreate(ctx context.Context, c *client, raw json.RawMessage) {
	var req drawCreateRequest
	if err := decodePayload(raw, &req); err != nil || req.SceneID == "" {
		h.sendError(ctx, c, "invalid draw.create payload")
		return
	}
	if !h.requireSceneInRoom(ctx, c, req.SceneID) {
		return
	}

	kind, ok := drawingKinds[req.Kind]
	if !ok {
		h.sendError(ctx, c, fmt.Sprintf("unknown drawing kind %q", req.Kind))
		return
	}
	// Freehand strokes can have any number of points; every other kind
	// is defined by exactly two — a line's start and end, or two
	// opposite corners of a rect's or ellipse's box.
	wantPoints := 2
	if (kind == store.DrawingKindFreehand && len(req.Points) < wantPoints) ||
		(kind != store.DrawingKindFreehand && len(req.Points) != wantPoints) {
		h.sendError(ctx, c, "invalid number of points for drawing kind")
		return
	}

	color := req.Color
	if color == "" {
		color = defaultDrawingColor
	}

	// The author is taken from the authenticated connection, never from
	// the payload — a client can't claim to have drawn something as
	// someone else.
	participantID := c.participant.ID
	drawing, err := h.store.CreateDrawing(req.SceneID, kind, req.Points, color, &participantID)
	if err != nil {
		slog.Error("ws: create drawing failed", "error", err)
		h.sendError(ctx, c, "failed to create drawing")
		return
	}

	h.broadcast(ctx, c.roomID, "drawing.created", drawingPayload(drawing))
}

type drawDeleteRequest struct {
	DrawingID string `json:"drawingId"`
}

// handleDrawDelete erases a drawing for everyone. A GM can erase
// anything on the map, including Players' work, since they're the one
// moderating the shared canvas; a Player can only erase what they drew
// themselves. A drawing with no recorded author (drawn before
// authorship was tracked, or by someone since removed from the room)
// belongs to nobody, so only a GM can clear it.
func (h *Hub) handleDrawDelete(ctx context.Context, c *client, raw json.RawMessage) {
	var req drawDeleteRequest
	if err := decodePayload(raw, &req); err != nil || req.DrawingID == "" {
		h.sendError(ctx, c, "invalid draw.delete payload")
		return
	}

	drawing, err := h.store.GetDrawing(req.DrawingID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("ws: lookup drawing failed", "error", err)
		}
		h.sendError(ctx, c, "drawing not found")
		return
	}
	// Scoped through the drawing's own scene rather than the requested
	// one, so a client can't reach into another room by ID.
	if !h.requireSceneInRoom(ctx, c, drawing.SceneID) {
		return
	}

	if c.participant.Role != store.RoleGM {
		if drawing.CreatedByParticipantID == nil || *drawing.CreatedByParticipantID != c.participant.ID {
			h.sendError(ctx, c, "you can only erase drawings you created")
			return
		}
	}

	if err := h.store.DeleteDrawing(drawing.ID); err != nil {
		slog.Error("ws: delete drawing failed", "error", err)
		h.sendError(ctx, c, "failed to erase drawing")
		return
	}

	h.broadcast(ctx, c.roomID, "drawing.deleted", map[string]any{"drawingId": drawing.ID})
}

type pingRequest struct {
	SceneID string  `json:"sceneId"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
}

// handlePing broadcasts a transient pointer-ping — it's never
// persisted or included in state.sync, since it only makes sense in
// the moment.
func (h *Hub) handlePing(ctx context.Context, c *client, raw json.RawMessage) {
	var req pingRequest
	if err := decodePayload(raw, &req); err != nil || req.SceneID == "" {
		h.sendError(ctx, c, "invalid ping payload")
		return
	}
	if !h.requireSceneInRoom(ctx, c, req.SceneID) {
		return
	}

	h.broadcast(ctx, c.roomID, "ping", map[string]any{
		"sceneId":         req.SceneID,
		"x":               req.X,
		"y":               req.Y,
		"participantName": c.participant.DisplayName,
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

	// There's no scene-switcher UI yet and the WS protocol has no
	// request/response correlation for the client to learn the new
	// scene's ID otherwise, so a newly created scene just becomes the
	// room's active one immediately.
	if err := h.store.SetActiveScene(c.roomID, scene.ID); err != nil {
		slog.Error("ws: activate new scene failed", "error", err)
		h.sendError(ctx, c, "created scene, but failed to activate it")
		return
	}

	h.broadcastSceneActivated(ctx, c, scene.ID)
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

	h.broadcastSceneActivated(ctx, c, req.SceneID)
}

// broadcastSceneActivated tells every client in the room about the
// newly active scene, including its tokens and fog — not just the bare
// ID — so they can render it immediately without another round trip.
// Hidden tokens are filtered out for non-GM recipients, so the two
// roles get different payloads (computed once each, not per client).
func (h *Hub) broadcastSceneActivated(ctx context.Context, initiator *client, sceneID string) {
	gmPayload, err := h.sceneStatePayload(sceneID, store.RoleGM)
	if err != nil {
		slog.Error("ws: load activated scene state failed", "error", err)
		h.sendError(ctx, initiator, "activated scene, but failed to load its state")
		return
	}
	playerPayload, err := h.sceneStatePayload(sceneID, store.RolePlayer)
	if err != nil {
		slog.Error("ws: load activated scene state failed", "error", err)
		h.sendError(ctx, initiator, "activated scene, but failed to load its state")
		return
	}

	h.broadcastPerClient(ctx, initiator.roomID, "scene.activated", func(recipient *client) any {
		if recipient.participant.Role == store.RoleGM {
			return gmPayload
		}
		return playerPayload
	})
}

const maxMessageLength = 2000

type chatSendRequest struct {
	Text string `json:"text"`
}

func (h *Hub) handleChatSend(ctx context.Context, c *client, raw json.RawMessage) {
	var req chatSendRequest
	if err := decodePayload(raw, &req); err != nil {
		h.sendError(ctx, c, "invalid chat.send payload")
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		h.sendError(ctx, c, "message text is required")
		return
	}
	if len(text) > maxMessageLength {
		h.sendError(ctx, c, fmt.Sprintf("message too long (max %d characters)", maxMessageLength))
		return
	}

	if strings.HasPrefix(text, "/") {
		h.handleSlashCommand(ctx, c, text)
		return
	}

	participantID := c.participant.ID
	h.postMessage(ctx, c, store.Message{
		RoomID:          c.roomID,
		ParticipantID:   &participantID,
		ParticipantName: c.participant.DisplayName,
		Kind:            store.MessageKindText,
		Body:            text,
	})
}

// handleSlashCommand dispatches a leading-"/" chat message to a known
// command. Unrecognized commands get an error back to the sender only
// — they never make it into the room's chat log.
func (h *Hub) handleSlashCommand(ctx context.Context, c *client, text string) {
	command, rest, _ := strings.Cut(strings.TrimPrefix(text, "/"), " ")
	rest = strings.TrimSpace(rest)

	switch strings.ToLower(command) {
	case "roll", "r":
		h.handleRollCommand(ctx, c, text, rest)
	default:
		h.sendError(ctx, c, fmt.Sprintf("unknown command \"/%s\"", command))
	}
}

func (h *Hub) handleRollCommand(ctx context.Context, c *client, rawText, expression string) {
	if expression == "" {
		h.sendError(ctx, c, "usage: /roll <expression>, e.g. /roll 2d6+3")
		return
	}

	result, err := dice.Roll(expression)
	if err != nil {
		h.sendError(ctx, c, fmt.Sprintf("invalid dice expression: %v", err))
		return
	}

	participantID := c.participant.ID
	h.postMessage(ctx, c, store.Message{
		RoomID:          c.roomID,
		ParticipantID:   &participantID,
		ParticipantName: c.participant.DisplayName,
		Kind:            store.MessageKindRoll,
		Body:            rawText,
		RollExpression:  &result.Expression,
		RollResult:      &result.Total,
		RollBreakdown:   &result.Breakdown,
	})
}

// postMessage persists and broadcasts a chat log entry.
func (h *Hub) postMessage(ctx context.Context, c *client, m store.Message) {
	msg, err := h.store.InsertMessage(m)
	if err != nil {
		slog.Error("ws: insert message failed", "error", err)
		h.sendError(ctx, c, "failed to send message")
		return
	}
	h.broadcast(ctx, c.roomID, "chat.posted", messagePayload(msg))
}
