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
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/google/uuid"

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
	arrived := h.register(c)
	// Both of these are declared before the unregister defer so they run
	// after it (defers run last-in-first-out): the leaving client
	// shouldn't be among the recipients of its own cleanup.
	var departed bool
	defer func() {
		if departed {
			h.announceDeparture(c)
		}
	}()
	defer h.endMeasurementOnDisconnect(c)
	defer func() { departed = h.unregister(c) }()

	ctx := r.Context()
	h.sendStateSync(ctx, c, room)
	if arrived {
		// Everyone *except* the arrival, which is the one broadcast in the
		// protocol that deliberately skips its own sender. Every other one
		// echoes back because the sender has something optimistic to
		// reconcile; here it has nothing — the state.sync just sent already
		// lists this participant among the connected, so an echo would say
		// something the client acted on a moment ago, and every test that
		// opens a connection would have to read past it.
		payload := participantPayload(participant)
		h.broadcastPerClient(ctx, room.ID, "participant.connected", func(recipient *client) any {
			if recipient == c {
				return nil
			}
			return payload
		})
	}

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return // client disconnected or context canceled
		}
		h.handleMessage(ctx, c, data)
	}
}

// register adds c to its room and reports whether this is the first
// connection its *participant* has open.
//
// Connections and people aren't the same thing: a second browser tab is
// a second client but the same person at the table, so only the first
// arriving and the last leaving are presence changes. Without that,
// opening a tab announces someone who was already here and closing it
// announces them gone while they're still looking at the map.
func (h *Hub) register(c *client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[c.roomID] == nil {
		h.rooms[c.roomID] = make(map[*client]struct{})
	}
	alreadyHere := h.participantConnectedLocked(c.roomID, c.participant.ID)
	h.rooms[c.roomID][c] = struct{}{}
	return !alreadyHere
}

// unregister removes c and reports whether that was the last connection
// its participant had open — see register for why that isn't the same
// question as whether a client went away.
func (h *Hub) unregister(c *client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms[c.roomID], c)
	stillHere := h.participantConnectedLocked(c.roomID, c.participant.ID)
	if len(h.rooms[c.roomID]) == 0 {
		delete(h.rooms, c.roomID)
	}
	return !stillHere
}

// participantConnectedLocked reports whether any connection in the room
// belongs to participantID. Caller holds h.mu.
func (h *Hub) participantConnectedLocked(roomID, participantID string) bool {
	for other := range h.rooms[roomID] {
		if other.participant.ID == participantID {
			return true
		}
	}
	return false
}

// connectedParticipantIDs lists who is connected to roomID right now,
// one entry per person however many tabs they have open.
func (h *Hub) connectedParticipantIDs(roomID string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	seen := make(map[string]struct{}, len(h.rooms[roomID]))
	ids := make([]string, 0, len(h.rooms[roomID]))
	for c := range h.rooms[roomID] {
		if _, dup := seen[c.participant.ID]; dup {
			continue
		}
		seen[c.participant.ID] = struct{}{}
		ids = append(ids, c.participant.ID)
	}
	return ids
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

// sendDrawingError is sendError for a failure the client can attribute
// to one drawing it has already rendered optimistically, so it knows
// exactly which stroke to take back. drawingID may be empty, for a
// client that let the server assign the id and so has nothing pending.
func (h *Hub) sendDrawingError(ctx context.Context, c *client, drawingID, message string) {
	if drawingID == "" {
		h.sendError(ctx, c, message)
		return
	}
	h.send(ctx, c, "error", map[string]string{"message": message, "drawingId": drawingID})
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
	case "token.update":
		h.handleTokenUpdate(ctx, c, env.Payload)
	case "token.delete":
		h.handleTokenDelete(ctx, c, env.Payload)
	case "fog.reveal":
		h.handleFogReveal(ctx, c, env.Payload)
	case "fog.hide":
		h.handleFogHide(ctx, c, env.Payload)
	case "fog.revealAll":
		h.handleFogRevealAll(ctx, c, env.Payload)
	case "fog.reset":
		h.handleFogReset(ctx, c, env.Payload)
	case "scene.create":
		h.handleSceneCreate(ctx, c, env.Payload)
	case "scene.setActive":
		h.handleSceneSetActive(ctx, c, env.Payload)
	case "scene.delete":
		h.handleSceneDelete(ctx, c, env.Payload)
	case "scene.setMap":
		h.handleSceneSetMap(ctx, c, env.Payload)
	case "chat.send":
		h.handleChatSend(ctx, c, env.Payload)
	case "chat.delete":
		h.handleChatDelete(ctx, c, env.Payload)
	case "draw.create":
		h.handleDrawCreate(ctx, c, env.Payload)
	case "draw.delete":
		h.handleDrawDelete(ctx, c, env.Payload)
	case "ping":
		h.handlePing(ctx, c, env.Payload)
	case "measure.update":
		h.handleMeasureUpdate(ctx, c, env.Payload)
	case "measure.end":
		h.handleMeasureEnd(ctx, c)
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

// sceneInRoom reports whether sceneID belongs to c's room — the check
// standing between a command and someone else's room. A scene that
// doesn't exist is not in the room either, so both answer false and
// callers report them identically rather than confirming which of the
// two it was.
func (h *Hub) sceneInRoom(c *client, sceneID string) bool {
	roomID, err := h.store.SceneRoomID(sceneID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("ws: lookup scene room failed", "error", err)
		}
		return false
	}
	return roomID == c.roomID
}

// requireSceneInRoom is sceneInRoom plus the error reply, for commands
// with nothing of their own to name in the failure.
func (h *Hub) requireSceneInRoom(ctx context.Context, c *client, sceneID string) bool {
	if !h.sceneInRoom(c, sceneID) {
		h.sendError(ctx, c, "scene not found")
		return false
	}
	return true
}

// requireAssetInRoom confirms a referenced asset ID is in this room's
// library, so scenes and tokens never point at a dangling ID — or at
// another room's art. A nil assetID (no image set) is always allowed.
//
// Membership in the library, not mere existence, is the check that
// matters: asset rows are global so that identical uploads share one
// stored file, which means every room's asset IDs live in one namespace.
// An ID that leaked from a private room would otherwise be usable in any
// room that learned it. "Not in your library" and "doesn't exist" get
// the same answer, so this can't be used to probe for what exists
// elsewhere.
func (h *Hub) requireAssetInRoom(ctx context.Context, c *client, assetID *string) bool {
	if assetID == nil {
		return true
	}
	inRoom, err := h.store.AssetInRoom(c.roomID, *assetID)
	if err != nil {
		slog.Error("ws: lookup room asset failed", "error", err)
		h.sendError(ctx, c, "asset not found")
		return false
	}
	if !inRoom {
		h.sendError(ctx, c, "asset not found")
		return false
	}
	return true
}

// requireOwnerInRoom confirms a token's owner is someone in this room. A
// nil ownerID (nobody owns it — the normal case for a monster) is always
// allowed.
//
// Same shape as requireAssetInRoom above, and for the same reason:
// participant IDs are unguessable but they aren't scoped, and a token
// handed to someone in another room would be a token whose owner nobody
// present could be shown, sitting behind whatever ownership permissions
// get built on top of it.
func (h *Hub) requireOwnerInRoom(ctx context.Context, c *client, ownerID *string) bool {
	if ownerID == nil {
		return true
	}
	inRoom, err := h.store.ParticipantInRoom(c.roomID, *ownerID)
	if err != nil {
		slog.Error("ws: lookup room participant failed", "error", err)
		h.sendError(ctx, c, "owner not found")
		return false
	}
	if !inRoom {
		h.sendError(ctx, c, "owner not found")
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

// Bounds on the free text a token carries. Neither is a storage
// concern — they're what keeps one client from writing a paragraph into
// a condition tag that every other client then has to draw over the
// map. Both are generous next to what stays readable at token size.
const (
	maxTrackerLabel    = 16
	maxConditionText   = 32
	maxTokenConditions = 12
)

// trackerRequest mirrors store.Tracker on the wire. Value is a pointer
// for the same reason it is there: null is an empty slot and 0 is a
// creature on nought hit points, and those are the two states a GM most
// needs told apart.
type trackerRequest struct {
	Label string `json:"label"`
	Value *int   `json:"value"`
}

// parseTrackers validates the slots a client sent and pads them out to
// the fixed count. Too many is an error rather than something to
// truncate quietly: a client sending four slots disagrees with the
// server about how many exist, and silently keeping the first three
// would drop whichever the user had just typed.
//
// A label with no value (a slot named ahead of being filled in) and a
// value with no label (a bare number) are both kept as sent. Which of
// them is worth drawing is the renderer's call, not the protocol's.
func parseTrackers(reqs []trackerRequest) ([]store.Tracker, error) {
	if len(reqs) > store.TrackerSlots {
		return nil, fmt.Errorf("a token has at most %d trackers", store.TrackerSlots)
	}
	out := make([]store.Tracker, store.TrackerSlots)
	for i, r := range reqs {
		label := strings.TrimSpace(r.Label)
		if utf8.RuneCountInString(label) > maxTrackerLabel {
			return nil, fmt.Errorf("a tracker label is at most %d characters", maxTrackerLabel)
		}
		out[i] = store.Tracker{Label: label, Value: r.Value}
	}
	return out, nil
}

// parseConditions trims, drops blanks and deduplicates. The dedupe is
// case-insensitive and keeps the first spelling seen: "Prone" and
// "prone" are one condition, and a token wearing both reads as a bug to
// everyone looking at it.
func parseConditions(raw []string) ([]string, error) {
	out := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, c := range raw {
		text := strings.TrimSpace(c)
		if text == "" {
			continue
		}
		if utf8.RuneCountInString(text) > maxConditionText {
			return nil, fmt.Errorf("a condition is at most %d characters", maxConditionText)
		}
		key := strings.ToLower(text)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, text)
	}
	// Counted after the dedupe, so a client that sent the same condition
	// twelve times is refused for the length it actually meant.
	if len(out) > maxTokenConditions {
		return nil, fmt.Errorf("a token has at most %d conditions", maxTokenConditions)
	}
	return out, nil
}

type tokenCreateRequest struct {
	// TokenID is optional and normally absent — the server mints one. A
	// client supplies it when undoing a deletion, where the token has to
	// come back under the id everyone else in the room still knows it by.
	TokenID            string  `json:"tokenId"`
	SceneID            string  `json:"sceneId"`
	Name               string  `json:"name"`
	ImageAssetID       *string `json:"imageAssetId"`
	X                  float64 `json:"x"`
	Y                  float64 `json:"y"`
	Width              float64 `json:"width"`
	Height             float64 `json:"height"`
	OwnerParticipantID *string `json:"ownerParticipantId"`
	Visibility         string  `json:"visibility"`
	// Normally absent — a token is created blank and its trackers filled
	// in afterwards. They're here for the same reason TokenID is: undoing
	// a deletion rebuilds the row from this payload alone, and a token
	// that came back on full health would be a worse bug than the misclick
	// the undo was for.
	Trackers   []trackerRequest `json:"trackers"`
	Conditions []string         `json:"conditions"`
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
	// Same canonical-form check draw.create makes on a client-chosen id,
	// and for the same reason: the id is echoed back, so anything but the
	// lowercase hyphenated spelling would come back as a different string
	// from the one the client is holding.
	if req.TokenID != "" && !isCanonicalUUID(req.TokenID) {
		h.sendError(ctx, c, "tokenId must be a canonical UUID")
		return
	}
	if !h.requireSceneInRoom(ctx, c, req.SceneID) {
		return
	}
	if !h.requireAssetInRoom(ctx, c, req.ImageAssetID) {
		return
	}
	if !h.requireOwnerInRoom(ctx, c, req.OwnerParticipantID) {
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

	trackers, err := parseTrackers(req.Trackers)
	if err != nil {
		h.sendError(ctx, c, err.Error())
		return
	}
	conditions, err := parseConditions(req.Conditions)
	if err != nil {
		h.sendError(ctx, c, err.Error())
		return
	}

	token, err := h.store.CreateToken(store.Token{
		ID:                 req.TokenID,
		SceneID:            req.SceneID,
		Name:               req.Name,
		ImageAssetID:       req.ImageAssetID,
		X:                  req.X,
		Y:                  req.Y,
		Width:              width,
		Height:             height,
		OwnerParticipantID: req.OwnerParticipantID,
		Visibility:         visibility,
		Trackers:           trackers,
		Conditions:         conditions,
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

// tokenUpdateRequest is token.create's payload minus the things an edit
// can't change: which scene it's on, and where it stands. Every editable
// field is sent every time rather than only the changed ones — a *string
// can't tell "left alone" from "cleared", and clearing a token's art is
// a real edit someone will want.
type tokenUpdateRequest struct {
	TokenID            string           `json:"tokenId"`
	Name               string           `json:"name"`
	ImageAssetID       *string          `json:"imageAssetId"`
	Width              float64          `json:"width"`
	Height             float64          `json:"height"`
	OwnerParticipantID *string          `json:"ownerParticipantId"`
	Visibility         string           `json:"visibility"`
	Trackers           []trackerRequest `json:"trackers"`
	Conditions         []string         `json:"conditions"`
}

// handleTokenUpdate edits a token in place.
//
// The role check is per field rather than the single gate at the top it
// used to be. A GM may change anything; a Player who *owns* the token
// may change its trackers and conditions and nothing else, because
// tracking your own damage is the one edit that shouldn't need asking
// for, while who can see the token, who owns it and what it looks like
// remain the GM's scene.
//
// The fields a Player may not touch are not rejected but ignored: the
// loaded token keeps them, exactly as it keeps a field this command
// doesn't carry at all. Rejecting instead would mean comparing every
// field the client echoed back against what's stored and calling any
// difference an attack, which turns a stale form into an error and buys
// nothing — the values are preserved either way.
func (h *Hub) handleTokenUpdate(ctx context.Context, c *client, raw json.RawMessage) {
	isGM := c.participant.Role == store.RoleGM

	var req tokenUpdateRequest
	// A name is required only of a GM. It's one of the GM-only fields, so
	// an owning Player's client has no reason to send one and shouldn't be
	// made to invent one to change its hit points.
	if err := decodePayload(raw, &req); err != nil || req.TokenID == "" || (isGM && req.Name == "") {
		h.sendError(ctx, c, "invalid token.update payload")
		return
	}

	// Loaded before anything is changed, for three things: the scene that
	// scopes the permission check, the visibility it's coming *from*, and
	// the fields this command doesn't carry — which are preserved by
	// editing the loaded token rather than building a fresh one.
	token, err := h.store.GetToken(req.TokenID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("ws: lookup token failed", "error", err)
		}
		h.sendError(ctx, c, "token not found")
		return
	}
	if !h.sceneInRoom(c, token.SceneID) {
		h.sendError(ctx, c, "token not found")
		return
	}

	if !isGM {
		// A hidden token is refused in the words of one that isn't there. A
		// Player is never told a hidden token exists, and an error that
		// separated "not yours" from "no such token" would be exactly that
		// telling — including for a hidden token they own, which they still
		// cannot see on their own map.
		if token.Visibility == store.VisibilityHidden {
			h.sendError(ctx, c, "token not found")
			return
		}
		if token.OwnerParticipantID == nil || *token.OwnerParticipantID != c.participant.ID {
			h.sendError(ctx, c, "you can only edit a token you own")
			return
		}
	}

	trackers, err := parseTrackers(req.Trackers)
	if err != nil {
		h.sendError(ctx, c, err.Error())
		return
	}
	conditions, err := parseConditions(req.Conditions)
	if err != nil {
		h.sendError(ctx, c, err.Error())
		return
	}

	wasHidden := token.Visibility == store.VisibilityHidden

	if isGM {
		if !h.requireAssetInRoom(ctx, c, req.ImageAssetID) {
			return
		}
		if !h.requireOwnerInRoom(ctx, c, req.OwnerParticipantID) {
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

		token.Name = req.Name
		token.ImageAssetID = req.ImageAssetID
		token.Width = width
		token.Height = height
		token.OwnerParticipantID = req.OwnerParticipantID
		token.Visibility = visibility
	}

	token.Trackers = trackers
	token.Conditions = conditions

	if err := h.store.UpdateToken(token); err != nil {
		slog.Error("ws: update token failed", "error", err)
		h.sendError(ctx, c, "failed to update token")
		return
	}

	// Withheld from Players while the token is hidden, as ever. A Player
	// who *can* see it gets the whole token rather than a diff, so the
	// same event serves the token they already had and one that has just
	// become visible to them — their client upserts on id.
	payload := tokenPayload(token)
	nowHidden := token.Visibility == store.VisibilityHidden
	h.broadcastPerClient(ctx, c.roomID, "token.updated", func(recipient *client) any {
		if nowHidden && recipient.participant.Role != store.RoleGM {
			return nil
		}
		return payload
	})

	// Going visible -> hidden is the one transition the update alone
	// can't express: Players are holding a token they must now stop
	// seeing, and the event that withheld itself from them told them
	// nothing. They get a deletion instead, which is exactly what has
	// happened as far as their map is concerned — the row lives on, but
	// hidden tokens have never been something a Player is told exists.
	if nowHidden && !wasHidden {
		gone := map[string]any{"tokenId": token.ID}
		h.broadcastPerClient(ctx, c.roomID, "token.deleted", func(recipient *client) any {
			if recipient.participant.Role == store.RoleGM {
				return nil
			}
			return gone
		})
	}
}

type tokenDeleteRequest struct {
	TokenID string `json:"tokenId"`
}

// handleTokenDelete takes a token off the map for everyone. GM-only,
// matching who may create one: anyone at the table can drag a token
// around, but only the GM decides what is on the map at all.
//
// Deliberately not the erase-style "your own work" rule draw.delete
// uses. A token isn't authored the way a stroke is — it's a piece of the
// GM's scene that a Player may be allowed to move — so there's no
// authorship to fall back on.
func (h *Hub) handleTokenDelete(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req tokenDeleteRequest
	if err := decodePayload(raw, &req); err != nil || req.TokenID == "" {
		h.sendError(ctx, c, "invalid token.delete payload")
		return
	}

	// Read before deleting, for two things the broadcast needs: the scene
	// that scopes the permission check, and the visibility that decides
	// who hears about it.
	token, err := h.store.GetToken(req.TokenID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("ws: lookup token failed", "error", err)
		}
		h.sendError(ctx, c, "token not found")
		return
	}
	// Scoped through the token's own scene rather than one named in the
	// payload, so a client can't reach into another room by id. A token
	// that doesn't exist and one belonging to someone else's room answer
	// identically.
	if !h.sceneInRoom(c, token.SceneID) {
		h.sendError(ctx, c, "token not found")
		return
	}

	if err := h.store.DeleteToken(token.ID); err != nil {
		slog.Error("ws: delete token failed", "error", err)
		h.sendError(ctx, c, "failed to delete token")
		return
	}

	// Withheld from Players when the token was hidden, exactly as its
	// creation was. They were never told it existed, and an id they have
	// never seen turning up in a deletion would tell them that it did.
	payload := map[string]any{"tokenId": token.ID}
	h.broadcastPerClient(ctx, c.roomID, "token.deleted", func(recipient *client) any {
		if token.Visibility == store.VisibilityHidden && recipient.participant.Role != store.RoleGM {
			return nil
		}
		return payload
	})
}

// fogCellsRequest is the payload of both fog.reveal and fog.hide, which
// are inverses over the same list of cells.
type fogCellsRequest struct {
	SceneID string          `json:"sceneId"`
	Cells   []store.FogCell `json:"cells"`
}

// fogSceneRequest is the payload of the whole-scene fog commands, which
// name a scene and nothing else.
type fogSceneRequest struct {
	SceneID string `json:"sceneId"`
}

func (h *Hub) handleFogReveal(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req fogCellsRequest
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

// handleFogHide puts revealed cells back under the cover, so a reveal
// painted over the wrong corridor can be taken back without resetting
// the scene. GM-only for the same reason revealing is: fog is the GM's
// control over what the room is allowed to see.
func (h *Hub) handleFogHide(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req fogCellsRequest
	if err := decodePayload(raw, &req); err != nil || req.SceneID == "" || len(req.Cells) == 0 {
		h.sendError(ctx, c, "invalid fog.hide payload")
		return
	}
	if !h.requireSceneInRoom(ctx, c, req.SceneID) {
		return
	}

	if err := h.store.HideCells(req.SceneID, req.Cells); err != nil {
		slog.Error("ws: hide fog failed", "error", err)
		h.sendError(ctx, c, "failed to hide fog")
		return
	}

	h.broadcast(ctx, c.roomID, "fog.hidden", map[string]any{
		"sceneId": req.SceneID,
		"cells":   req.Cells,
	})
}

// handleFogRevealAll uncovers the whole scene at once — for a map that
// doesn't want fog, or the moment an encounter ends.
//
// It materialises every cell rather than setting a scene-level
// "everything revealed" flag. Fog's only representation is the set of
// revealed cells, and a flag would need reconciling with that set the
// first time the GM hid a single cell afterwards. Materialising also
// lets this broadcast the existing fog.revealed rather than an event of
// its own, which keeps the server the one place that decides what cells
// a scene has: a client computing them from the scene's dimensions
// would have to agree with this exactly or drift on the next reload.
func (h *Hub) handleFogRevealAll(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req fogSceneRequest
	if err := decodePayload(raw, &req); err != nil || req.SceneID == "" {
		h.sendError(ctx, c, "invalid fog.revealAll payload")
		return
	}
	if !h.requireSceneInRoom(ctx, c, req.SceneID) {
		return
	}

	scene, err := h.store.GetScene(req.SceneID)
	if err != nil {
		slog.Error("ws: load scene for reveal-all failed", "error", err)
		h.sendError(ctx, c, "failed to reveal fog")
		return
	}

	cells, err := sceneFogCells(scene)
	if err != nil {
		// Bounds the GM can see and fix, not an internal failure, so the
		// reason goes to them verbatim.
		h.sendError(ctx, c, err.Error())
		return
	}

	if err := h.store.RevealCells(req.SceneID, cells); err != nil {
		slog.Error("ws: reveal all fog failed", "error", err)
		h.sendError(ctx, c, "failed to reveal fog")
		return
	}

	h.broadcast(ctx, c.roomID, "fog.revealed", map[string]any{
		"sceneId": req.SceneID,
		"cells":   cells,
	})
}

// handleFogReset returns a scene to fully covered, the state it starts
// in. There's no undo for this — see the note on the toolbar button.
func (h *Hub) handleFogReset(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req fogSceneRequest
	if err := decodePayload(raw, &req); err != nil || req.SceneID == "" {
		h.sendError(ctx, c, "invalid fog.reset payload")
		return
	}
	if !h.requireSceneInRoom(ctx, c, req.SceneID) {
		return
	}

	if err := h.store.ClearFog(req.SceneID); err != nil {
		slog.Error("ws: reset fog failed", "error", err)
		h.sendError(ctx, c, "failed to reset fog")
		return
	}

	h.broadcast(ctx, c.roomID, "fog.reset", map[string]any{
		"sceneId": req.SceneID,
	})
}

// maxRevealAllCells caps what one fog.revealAll may materialise. The
// count grows with the product of the map's dimensions in grid squares,
// and every cell is both a row inserted and an entry in the payload
// every client receives. 40,000 is a 200x200 grid — far past any map
// anyone actually plays on, and still small enough to insert and
// broadcast without stalling the room.
const maxRevealAllCells = 40_000

// sceneFogCells enumerates every cell inside a scene's bounds, indexed
// the way the client indexes a painted one — floor(pixel / gridSize)
// from the origin — so a revealed-everything scene and a hand-painted
// one agree on what a cell is.
func sceneFogCells(scene store.Scene) ([]store.FogCell, error) {
	if scene.GridSize <= 0 || scene.Width <= 0 || scene.Height <= 0 {
		return nil, errors.New("this scene has no map bounds to reveal")
	}

	// Rounded up, so a map whose last row of squares is clipped still
	// gets that row revealed rather than leaving a covered strip.
	cols := (scene.Width + scene.GridSize - 1) / scene.GridSize
	rows := (scene.Height + scene.GridSize - 1) / scene.GridSize
	if cols*rows > maxRevealAllCells {
		return nil, fmt.Errorf("this scene covers %d cells, too many to reveal at once", cols*rows)
	}

	cells := make([]store.FogCell, 0, cols*rows)
	for y := range rows {
		for x := range cols {
			cells = append(cells, store.FogCell{X: x, Y: y})
		}
	}
	return cells, nil
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
	// DrawingID is chosen by the client, which has already drawn the
	// stroke under it rather than waiting for this round trip. Optional:
	// a client that doesn't care gets a server-generated one.
	DrawingID string        `json:"drawingId"`
	SceneID   string        `json:"sceneId"`
	Kind      string        `json:"kind"`
	Points    []store.Point `json:"points"`
	Color     string        `json:"color"`
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
	// From here on the client knows which stroke it asked for, so every
	// rejection names it — that's what lets an optimistic client take
	// the failed one back off the map again.
	if req.DrawingID != "" && !isCanonicalUUID(req.DrawingID) {
		h.sendDrawingError(ctx, c, req.DrawingID, "drawingId must be a canonical UUID")
		return
	}
	if !h.sceneInRoom(c, req.SceneID) {
		h.sendDrawingError(ctx, c, req.DrawingID, "scene not found")
		return
	}

	kind, ok := drawingKinds[req.Kind]
	if !ok {
		h.sendDrawingError(ctx, c, req.DrawingID, fmt.Sprintf("unknown drawing kind %q", req.Kind))
		return
	}
	// Freehand strokes can have any number of points; every other kind
	// is defined by exactly two — a line's start and end, or two
	// opposite corners of a rect's or ellipse's box.
	wantPoints := 2
	if (kind == store.DrawingKindFreehand && len(req.Points) < wantPoints) ||
		(kind != store.DrawingKindFreehand && len(req.Points) != wantPoints) {
		h.sendDrawingError(ctx, c, req.DrawingID, "invalid number of points for drawing kind")
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
	drawing, err := h.store.CreateDrawing(store.Drawing{
		ID:                     req.DrawingID,
		SceneID:                req.SceneID,
		Kind:                   kind,
		Points:                 req.Points,
		Color:                  color,
		CreatedByParticipantID: &participantID,
	})
	if err != nil {
		slog.Error("ws: create drawing failed", "error", err)
		h.sendDrawingError(ctx, c, req.DrawingID, "failed to create drawing")
		return
	}

	h.broadcast(ctx, c.roomID, "drawing.created", drawingPayload(drawing))
}

// isCanonicalUUID accepts only the lowercase hyphenated form, so the id
// echoed back to the client is byte-identical to the one it sent and
// matches the stroke it already has on screen. uuid.Parse alone is too
// lenient for that — it also takes braced and URN spellings.
func isCanonicalUUID(s string) bool {
	parsed, err := uuid.Parse(s)
	return err == nil && parsed.String() == s
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

	// As with draw.create, a rejection names the drawing so a client that
	// has already taken the stroke off its own map can put it back.
	drawing, err := h.store.GetDrawing(req.DrawingID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("ws: lookup drawing failed", "error", err)
		}
		h.sendDrawingError(ctx, c, req.DrawingID, "drawing not found")
		return
	}
	// Scoped through the drawing's own scene rather than the requested
	// one, so a client can't reach into another room by ID.
	if !h.sceneInRoom(c, drawing.SceneID) {
		h.sendDrawingError(ctx, c, req.DrawingID, "drawing not found")
		return
	}

	if c.participant.Role != store.RoleGM {
		if drawing.CreatedByParticipantID == nil || *drawing.CreatedByParticipantID != c.participant.ID {
			h.sendDrawingError(ctx, c, req.DrawingID, "you can only erase drawings you created")
			return
		}
	}

	if err := h.store.DeleteDrawing(drawing.ID); err != nil {
		slog.Error("ws: delete drawing failed", "error", err)
		h.sendDrawingError(ctx, c, req.DrawingID, "failed to erase drawing")
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

type measureUpdateRequest struct {
	SceneID string      `json:"sceneId"`
	Kind    string      `json:"kind"`
	From    store.Point `json:"from"`
	To      store.Point `json:"to"`
	// Only a line has a width a drag can't express. Feet rather than
	// world units, so it means the same thing on a scene with a
	// different grid size.
	WidthFeet float64 `json:"widthFeet"`
}

// measureKinds is the set of shapes a measurement may be, and doubles as
// the recognised-kind check. A plain distance line is the original and
// stays the default, so a client that doesn't know about templates —
// or an older one — keeps working unchanged.
//
// The four area shapes are what the 2024 PHB's six flatten to on a
// top-down map: sphere, cylinder and emanation are all a circle.
var measureKinds = map[string]bool{
	"distance": true,
	"circle":   true,
	"cone":     true,
	"line":     true,
	"cube":     true,
}

const defaultMeasureKind = "distance"

// handleMeasureUpdate relays where a participant is currently dragging a
// measurement. Like ping it is never persisted — a measurement only
// means anything while it's being made — but unlike ping it is a
// continuous gesture, so each participant has at most one in flight and
// every update replaces the last. Recipients key on participantId; the
// gesture ends with measure.end.
//
// Neither the distance nor the template's outline is computed here: the
// two endpoints (plus a width, for a line) are all anyone needs, and
// every client already knows the scene's grid size, so sending a number
// or a polygon as well would just be a second source of truth to keep in
// step with the shape. Snapping is applied client-side before sending
// for the same reason — the points that arrive here are final, and a
// recipient never needs to know which convention produced them.
func (h *Hub) handleMeasureUpdate(ctx context.Context, c *client, raw json.RawMessage) {
	var req measureUpdateRequest
	if err := decodePayload(raw, &req); err != nil || req.SceneID == "" {
		h.sendError(ctx, c, "invalid measure.update payload")
		return
	}
	if req.Kind == "" {
		req.Kind = defaultMeasureKind
	}
	if !measureKinds[req.Kind] {
		h.sendError(ctx, c, fmt.Sprintf("unknown measurement kind %q", req.Kind))
		return
	}
	if !h.requireSceneInRoom(ctx, c, req.SceneID) {
		return
	}

	h.broadcast(ctx, c.roomID, "measure.updated", map[string]any{
		"participantId":   c.participant.ID,
		"participantName": c.participant.DisplayName,
		"sceneId":         req.SceneID,
		"kind":            req.Kind,
		"from":            req.From,
		"to":              req.To,
		"widthFeet":       req.WidthFeet,
	})
}

// handleMeasureEnd takes a participant's measurement back off everyone's
// map. It carries no payload: a participant has only one measurement at
// a time, and the connection says whose it is.
func (h *Hub) handleMeasureEnd(ctx context.Context, c *client) {
	h.broadcastMeasureEnded(ctx, c)
}

func (h *Hub) broadcastMeasureEnded(ctx context.Context, c *client) {
	h.broadcast(ctx, c.roomID, "measure.ended", map[string]any{"participantId": c.participant.ID})
}

const measureCleanupTimeout = 5 * time.Second

// endMeasurementOnDisconnect clears a measurement left behind by a
// client that dropped mid-drag, which would otherwise hang on every
// other map until the scene changed. Sent unconditionally rather than
// tracked per client: a measure.ended for someone who wasn't measuring
// is a no-op for recipients, and that's cheaper than the bookkeeping.
//
// The request context is already canceled by the time a connection
// drops, so this gets a fresh one of its own.
func (h *Hub) endMeasurementOnDisconnect(c *client) {
	ctx, cancel := context.WithTimeout(context.Background(), measureCleanupTimeout)
	defer cancel()
	h.broadcastMeasureEnded(ctx, c)
}

// announceDeparture tells the room someone has gone. Only the id: they
// stay on the roster, which is everyone who has *ever* joined — what
// changed is whether they're at the table, not whether they exist.
//
// Needs a fresh context for the same reason the measurement cleanup
// above does: the request context is already canceled by the time a
// connection drops, so writing on it would go nowhere.
func (h *Hub) announceDeparture(c *client) {
	ctx, cancel := context.WithTimeout(context.Background(), measureCleanupTimeout)
	defer cancel()
	h.broadcast(ctx, c.roomID, "participant.disconnected", map[string]any{
		"participantId": c.participant.ID,
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
	if !h.requireAssetInRoom(ctx, c, req.MapAssetID) {
		return
	}
	if req.GridSize == 0 {
		req.GridSize = 70
	}

	room, err := h.store.GetRoomByID(c.roomID)
	if err != nil {
		slog.Error("ws: load room for scene create failed", "error", err)
		h.sendError(ctx, c, "failed to create scene")
		return
	}

	scene, err := h.store.CreateScene(c.roomID, req.Name, req.MapAssetID, req.GridSize, req.Width, req.Height)
	if err != nil {
		slog.Error("ws: create scene failed", "error", err)
		h.sendError(ctx, c, "failed to create scene")
		return
	}

	h.broadcast(ctx, c.roomID, "scene.created", map[string]any{"scene": scenePayload(scene)})

	// A new scene only takes over the room when there wasn't one to take
	// over from. Building the *second* scene mid-session is prep work —
	// yanking the party off the map they're standing on to look at an
	// empty one is not what a GM meant by "New scene". They switch to it
	// deliberately, through the picker. (Before that picker existed this
	// activated unconditionally, because activation was the only way to
	// ever reach a scene again.)
	if room.ActiveSceneID != nil {
		return
	}
	if err := h.store.SetActiveScene(c.roomID, scene.ID); err != nil {
		slog.Error("ws: activate new scene failed", "error", err)
		h.sendError(ctx, c, "created scene, but failed to activate it")
		return
	}

	h.broadcastSceneActivated(ctx, c, scene.ID)
}

type sceneDeleteRequest struct {
	SceneID string `json:"sceneId"`
}

// handleSceneDelete removes a scene and everything on it. The room's
// active scene is refused: `room.active_scene_id` has no foreign key to
// clean it up, so deleting it would leave every client staring at a
// scene the server can no longer load. Switching away first is one
// click, and makes the destruction deliberate.
func (h *Hub) handleSceneDelete(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req sceneDeleteRequest
	if err := decodePayload(raw, &req); err != nil || req.SceneID == "" {
		h.sendError(ctx, c, "invalid scene.delete payload")
		return
	}
	if !h.requireSceneInRoom(ctx, c, req.SceneID) {
		return
	}

	room, err := h.store.GetRoomByID(c.roomID)
	if err != nil {
		slog.Error("ws: load room for scene delete failed", "error", err)
		h.sendError(ctx, c, "failed to delete scene")
		return
	}
	if room.ActiveSceneID != nil && *room.ActiveSceneID == req.SceneID {
		h.sendError(ctx, c, "switch to another scene before deleting this one")
		return
	}

	if err := h.store.DeleteScene(req.SceneID); err != nil {
		slog.Error("ws: delete scene failed", "error", err)
		h.sendError(ctx, c, "failed to delete scene")
		return
	}

	h.broadcast(ctx, c.roomID, "scene.deleted", map[string]any{"sceneId": req.SceneID})
}

type sceneSetMapRequest struct {
	SceneID    string  `json:"sceneId"`
	MapAssetID *string `json:"mapAssetId"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
}

// handleSceneSetMap swaps the art under a scene without disturbing what
// has been placed on it. Deliberately *not* a scene.activated broadcast
// even for the active scene: that carries the full picture and makes
// clients treat it as a scene change, throwing away undo history and
// in-flight gestures. Only the scene itself changed, so only the scene
// is sent, and the tokens, fog and drawings already on screen stay
// exactly where they are — which is the whole point of replacing a map
// rather than building a new scene.
func (h *Hub) handleSceneSetMap(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req sceneSetMapRequest
	if err := decodePayload(raw, &req); err != nil || req.SceneID == "" {
		h.sendError(ctx, c, "invalid scene.setMap payload")
		return
	}
	if !h.requireSceneInRoom(ctx, c, req.SceneID) {
		return
	}
	if !h.requireAssetInRoom(ctx, c, req.MapAssetID) {
		return
	}

	if err := h.store.SetSceneMap(req.SceneID, req.MapAssetID, req.Width, req.Height); err != nil {
		slog.Error("ws: set scene map failed", "error", err)
		h.sendError(ctx, c, "failed to replace the map")
		return
	}

	scene, err := h.store.GetScene(req.SceneID)
	if err != nil {
		slog.Error("ws: load scene after map swap failed", "error", err)
		h.sendError(ctx, c, "replaced the map, but failed to load the scene")
		return
	}

	h.broadcast(ctx, c.roomID, "scene.updated", map[string]any{"scene": scenePayload(scene)})
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

// postMessage persists and broadcasts a chat log entry. viewerParticipantID
// is irrelevant to messagePayload until the message is deleted, so a fresh
// message goes out identically to everyone.
func (h *Hub) postMessage(ctx context.Context, c *client, m store.Message) {
	msg, err := h.store.InsertMessage(m)
	if err != nil {
		slog.Error("ws: insert message failed", "error", err)
		h.sendError(ctx, c, "failed to send message")
		return
	}
	h.broadcast(ctx, c.roomID, "chat.posted", messagePayload(msg, ""))
}

type chatDeleteRequest struct {
	MessageID string `json:"messageId"`
}

// handleChatDelete folds two delete stages into one command. The first
// call on a message soft-deletes it: the author and whoever just
// deleted it keep seeing the original content struck through, everyone
// else gets a generic placeholder — chat.deleted goes out per-client,
// same technique as a hidden token's broadcast. A second call, sent
// again once it's already deleted, purges the row outright for
// everyone. Authorization is the same author-or-GM check both times,
// against the row's original author, which is why the row has to
// survive the first stage rather than being wiped immediately: a purge
// with nothing left to check authorship against would have to fall back
// to GM-only, silently taking the second click away from the Player who
// made the first.
func (h *Hub) handleChatDelete(ctx context.Context, c *client, raw json.RawMessage) {
	var req chatDeleteRequest
	if err := decodePayload(raw, &req); err != nil || req.MessageID == "" {
		h.sendError(ctx, c, "invalid chat.delete payload")
		return
	}

	msg, err := h.store.GetMessage(req.MessageID)
	// A message in another room answers identically to one that doesn't
	// exist, same as sceneInRoom elsewhere — a client can't use this to
	// probe what's in a room it isn't in.
	if err != nil || msg.RoomID != c.roomID {
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			slog.Error("ws: lookup message failed", "error", err)
		}
		h.sendError(ctx, c, "message not found")
		return
	}

	if c.participant.Role != store.RoleGM {
		if msg.ParticipantID == nil || *msg.ParticipantID != c.participant.ID {
			h.sendError(ctx, c, "you can only delete messages you sent")
			return
		}
	}

	if msg.DeletedAt == nil {
		if err := h.store.SoftDeleteMessage(msg.ID, c.participant.ID); err != nil {
			slog.Error("ws: soft-delete message failed", "error", err)
			h.sendError(ctx, c, "failed to delete message")
			return
		}
		// messagePayload only redacts once it sees DeletedAt set, so the
		// in-memory copy needs both fields the store just persisted —
		// its exact value doesn't matter, only that it's non-nil.
		deletedAt := time.Now().UTC().Format(time.RFC3339Nano)
		msg.DeletedAt = &deletedAt
		deletedBy := c.participant.ID
		msg.DeletedByParticipantID = &deletedBy
		h.broadcastPerClient(ctx, c.roomID, "chat.deleted", func(recipient *client) any {
			return messagePayload(msg, recipient.participant.ID)
		})
		return
	}

	if err := h.store.DeleteMessage(msg.ID); err != nil {
		slog.Error("ws: purge message failed", "error", err)
		h.sendError(ctx, c, "failed to delete message")
		return
	}
	h.broadcast(ctx, c.roomID, "chat.purged", map[string]any{"messageId": msg.ID})
}
