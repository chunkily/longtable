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

// How long someone may be gone before the room is told they left.
//
// Presence used to be a read of the socket map, which cannot tell "gone"
// from "coming straight back" — and the reconnect backoff starts at half
// a second, so every blip took a badge off every rail and put it back.
// A phone locking its screen does it; a page reload does it.
//
// Long enough to cover a reload and a wobble, short enough that someone
// who has actually shut the laptop leaves the list while anyone still
// cares. Nothing about it is exact, which is why `-departure-grace`
// exists: the right answer is the table's wifi rather than ours.
const DefaultDepartureGrace = 30 * time.Second

type Hub struct {
	store *store.Store

	mu    sync.Mutex
	rooms map[string]map[*client]struct{}
	// Participants whose last connection has closed but whose grace
	// period has not expired: still present as far as the room is
	// concerned, and holding the timer that will say otherwise. Keyed by
	// room, then participant.
	//
	// Plain maps under mu rather than sync.Map for the same reason rooms
	// is: every read here is already inside the lock that the socket map
	// needs anyway.
	departing map[string]map[string]*time.Timer
	// Set from -departure-grace, and shortened by tests that would
	// otherwise spend half a minute proving a timer fired.
	departureGrace time.Duration
}

// NewHub takes the grace period rather than reading the constant, so the
// value a Host set on the command line is the one every room uses. Zero
// or negative falls back to the default: a grace of nothing is the
// flicker this was built to remove, and is more likely a typo than a
// preference.
func NewHub(s *store.Store, departureGrace time.Duration) *Hub {
	if departureGrace <= 0 {
		departureGrace = DefaultDepartureGrace
	}
	return &Hub{
		store:          s,
		rooms:          make(map[string]map[*client]struct{}),
		departing:      make(map[string]map[string]*time.Timer),
		departureGrace: departureGrace,
	}
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
	// unregister no longer announces anything itself: a departure is
	// broadcast by a timer, once the grace period has passed without the
	// participant coming back. That also means the ordering this used to
	// depend on — the leaver not being among the recipients of its own
	// cleanup — is now free, since it has left the socket map long before
	// the announcement goes out.
	defer h.endMeasurementOnDisconnect(c)
	defer h.unregister(c)

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
		// The chat log's copy of the same fact, and the durable one: a
		// badge says who is here now, this says who turned up tonight. It
		// goes to the arrival as well — unlike the badge, they have no
		// state.sync entry that already told them.
		h.postSystemMessage(ctx, room.ID, participant, store.SystemEventJoined)
	}

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return // client disconnected or context canceled
		}
		h.handleMessage(ctx, c, data)
	}
}

// register adds c to its room and reports whether the room should be
// told somebody arrived.
//
// Connections and people aren't the same thing: a second browser tab is
// a second client but the same person at the table, so only the first
// arriving and the last leaving are presence changes. Without that,
// opening a tab announces someone who was already here and closing it
// announces them gone while they're still looking at the map.
//
// A connection landing inside a pending departure is a **resumption**,
// and returns false: the room was never told this participant left, so
// telling it they arrived would announce a change that never happened —
// and, with a chat log listening, write "Bob left" and "Bob joined" into
// it for a wobble on the wifi. Cancelling the timer is the whole of what
// a reconnect has to do.
func (h *Hub) register(c *client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[c.roomID] == nil {
		h.rooms[c.roomID] = make(map[*client]struct{})
	}
	alreadyHere := h.participantConnectedLocked(c.roomID, c.participant.ID)
	h.rooms[c.roomID][c] = struct{}{}

	if timer, pending := h.departing[c.roomID][c.participant.ID]; pending {
		timer.Stop()
		h.clearDepartureLocked(c.roomID, c.participant.ID)
		return false
	}
	return !alreadyHere
}

// unregister removes c and, if that was its participant's last
// connection, starts the grace period. It never announces anything
// itself — see departureGrace, and announceDeparture for what happens
// when the timer runs out.
//
// The room map entry is deliberately kept alive while anyone is
// departing: dropping it would take the pending timer's room with it,
// and the last person to leave a room is exactly the case that has to
// still work.
func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms[c.roomID], c)
	if h.participantConnectedLocked(c.roomID, c.participant.ID) {
		h.pruneRoomLocked(c.roomID)
		return
	}

	roomID, participant := c.roomID, c.participant
	if h.departing[roomID] == nil {
		h.departing[roomID] = make(map[string]*time.Timer)
	}
	h.departing[roomID][participant.ID] = time.AfterFunc(h.departureGrace, func() {
		h.finishDeparture(roomID, participant)
	})
}

// finishDeparture runs when a grace period expires without the
// participant coming back: they are gone, and now the room hears about
// it.
//
// It re-checks under the lock rather than trusting the timer, because
// time.AfterFunc has no way to un-fire. A timer that was stopped a
// microsecond too late is already running this function while register
// holds the lock, so the entry it is looking for is the proof it is
// still wanted.
func (h *Hub) finishDeparture(roomID string, participant store.Participant) {
	h.mu.Lock()
	if _, pending := h.departing[roomID][participant.ID]; !pending {
		h.mu.Unlock()
		return
	}
	h.clearDepartureLocked(roomID, participant.ID)
	h.pruneRoomLocked(roomID)
	h.mu.Unlock()

	h.announceDeparture(roomID, participant)
}

// clearDepartureLocked forgets a pending departure, and the room's map
// with it once the last one goes. Caller holds h.mu.
func (h *Hub) clearDepartureLocked(roomID, participantID string) {
	delete(h.departing[roomID], participantID)
	if len(h.departing[roomID]) == 0 {
		delete(h.departing, roomID)
	}
}

// pruneRoomLocked drops an empty room from the socket map, unless
// somebody is still inside their grace period there. Caller holds h.mu.
func (h *Hub) pruneRoomLocked(roomID string) {
	if len(h.rooms[roomID]) == 0 && len(h.departing[roomID]) == 0 {
		delete(h.rooms, roomID)
	}
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

// ConnectedParticipantIDs lists who is present in roomID right now, one
// entry per person however many tabs or devices they have open.
//
// The dedupe is what makes "two devices on one seat is one person" true
// rather than merely intended: it keys on the seat, so a phone and a
// laptop signed into the same seat collapse to one entry here exactly as
// two tabs always did. Exported for the pre-join seat list, which shows
// whether anyone is sitting in a chair right now — a live question only
// the hub can answer, since presence is never written down.
//
// **Anyone inside their grace period counts as present**, which is what
// keeps a client that syncs mid-window agreeing with the room it just
// joined. Leaving them out would be worse than the flicker this replaces:
// a resumption broadcasts nothing by design, so the arrival that would
// have corrected that client's list never comes, and the person stays
// missing from one rail until something else resyncs it.
func (h *Hub) ConnectedParticipantIDs(roomID string) []string {
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
	for participantID := range h.departing[roomID] {
		if _, dup := seen[participantID]; dup {
			continue
		}
		seen[participantID] = struct{}{}
		ids = append(ids, participantID)
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
	case "participant.setColor":
		h.handleParticipantSetColor(ctx, c, env.Payload)
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
	case "room.setOwnerOnlyMovement":
		h.handleRoomSetOwnerOnlyMovement(ctx, c, env.Payload)
	case "initiative.add":
		h.handleInitiativeAdd(ctx, c, env.Payload)
	case "initiative.update":
		h.handleInitiativeUpdate(ctx, c, env.Payload)
	case "initiative.remove":
		h.handleInitiativeRemove(ctx, c, env.Payload)
	case "initiative.reorder":
		h.handleInitiativeReorder(ctx, c, env.Payload)
	case "initiative.advance":
		h.handleInitiativeAdvance(ctx, c, env.Payload)
	case "initiative.clear":
		h.handleInitiativeClear(ctx, c)
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

// handleTokenMove drags a token to a new square.
//
// Open to everyone by default — an open table is what Longtable is
// (ADR-0007) — but a room can be locked to owner-only movement, and then
// a non-GM may move only a token they own. The check lives here rather
// than anywhere else on purpose: **undoing a move is an ordinary
// token.move**, so whatever this handler enforces governs the undo for
// free, and a Player can't walk a locked token backwards through their
// history.
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

	if !h.mayMoveToken(ctx, c, req.TokenID) {
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

// mayMoveToken answers the room's movement lock, reporting the refusal
// itself so the caller reads as one line.
//
// Nothing is loaded at all in the common case: a GM is allowed
// regardless, and an unlocked room is the default, so an ordinary table
// pays one cheap room read per drag and no token read.
func (h *Hub) mayMoveToken(ctx context.Context, c *client, tokenID string) bool {
	if c.participant.Role == store.RoleGM {
		return true
	}

	room, err := h.store.GetRoomByID(c.roomID)
	if err != nil {
		slog.Error("ws: lookup room for move failed", "error", err)
		h.sendError(ctx, c, "failed to move token")
		return false
	}
	if !room.OwnerOnlyMovement {
		return true
	}

	token, err := h.store.GetToken(tokenID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("ws: lookup token failed", "error", err)
		}
		h.sendError(ctx, c, "token not found")
		return false
	}
	// A hidden token is refused in the words of one that isn't there,
	// including to its own owner — the same sentence handleTokenUpdate and
	// handleTokenDelete use, and for the same reason: a GM can prep an
	// ambush with a Player's character, and an error that told the two
	// apart is how they'd find out. Only reachable with the lock on; an
	// open room has never checked visibility on a move, and turning that
	// into a refusal here would be a second, unasked-for feature.
	if token.Visibility == store.VisibilityHidden {
		h.sendError(ctx, c, "token not found")
		return false
	}
	if token.OwnerParticipantID == nil || *token.OwnerParticipantID != c.participant.ID {
		h.sendError(ctx, c, "this table only lets you move your own tokens")
		return false
	}
	return true
}

type roomOwnerOnlyMovementRequest struct {
	OwnerOnlyMovement bool `json:"ownerOnlyMovement"`
}

// handleRoomSetOwnerOnlyMovement flips the room's movement lock. GM-only,
// and takes effect immediately for everyone: the broadcast is what makes
// a Player's tokens stop being draggable mid-session rather than at their
// next reload.
type participantSetColorRequest struct {
	Color string `json:"color"`
}

// handleParticipantSetColor changes the caller's own identity colour.
//
// Its own, and no argument about whose: the seat updated is
// c.participant.ID, taken from the connection, so there is no
// participantId in the payload to get wrong or to forge. That is the
// same rule every handler here follows and the reason this needs no
// permission check — a Player changing a colour is changing theirs by
// construction.
//
// Broadcast to everyone including the sender. Chat names and pings
// resolve colour from the roster, so a client that updated its own copy
// optimistically and one that waited would disagree until the next sync;
// one event, one source, and the sender's own log recolours with
// everybody else's.
func (h *Hub) handleParticipantSetColor(ctx context.Context, c *client, raw json.RawMessage) {
	var req participantSetColorRequest
	if err := decodePayload(raw, &req); err != nil {
		h.sendError(ctx, c, "invalid participant.setColor payload")
		return
	}
	if !store.ValidIdentityColor(req.Color) {
		// The set lives in Go rather than in a CHECK constraint, and this
		// value reaches a style attribute on every other client's screen.
		h.sendError(ctx, c, "unknown colour")
		return
	}

	if err := h.store.SetParticipantColor(c.roomID, c.participant.ID, req.Color); err != nil {
		slog.Error("ws: set participant colour failed", "error", err)
		h.sendError(ctx, c, "failed to change your colour")
		return
	}

	// The connection's own copy, so anything later in this session that
	// reads c.participant sees the new colour too.
	c.participant.Color = req.Color
	h.broadcast(ctx, c.roomID, "participant.updated", participantPayload(c.participant))
}

func (h *Hub) handleRoomSetOwnerOnlyMovement(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req roomOwnerOnlyMovementRequest
	if err := decodePayload(raw, &req); err != nil {
		h.sendError(ctx, c, "invalid room.setOwnerOnlyMovement payload")
		return
	}

	if err := h.store.SetOwnerOnlyMovement(c.roomID, req.OwnerOnlyMovement); err != nil {
		slog.Error("ws: set owner-only movement failed", "error", err)
		h.sendError(ctx, c, "failed to save that setting")
		return
	}

	room, err := h.store.GetRoomByID(c.roomID)
	if err != nil {
		slog.Error("ws: lookup room after settings change failed", "error", err)
		h.sendError(ctx, c, "failed to save that setting")
		return
	}
	// The whole room rather than the one field that changed, like
	// scene.updated carries the whole scene: the next setting to land here
	// then needs no new event, and a client that reloads sees the same
	// shape it got from state.sync.
	h.broadcast(ctx, c.roomID, "room.updated", map[string]any{"room": roomPayload(room)})
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

// maxTokensPerCreate caps a batch. A stepper is a convenience, not a
// permission — the command has to refuse five hundred monkeys whatever
// sent it, since each one is a row, a broadcast to every client and a
// group the canvas rebuilds.
const maxTokensPerCreate = 20

type tokenCreateRequest struct {
	// TokenIDs is optional and normally absent — the server mints them.
	// A client supplies them for two reasons, both about an id it already
	// holds: undoing a deletion, where the token has to come back under
	// the id everyone else in the room still knows it by, and creating a
	// batch it wants on its own undo stack, where knowing the ids up front
	// is what lets it record one entry per token without having to work
	// out which of the echoes arriving are its own. Must be exactly Count
	// long when present.
	TokenIDs           []string `json:"tokenIds"`
	SceneID            string   `json:"sceneId"`
	Name               string   `json:"name"`
	ImageAssetID       *string  `json:"imageAssetId"`
	X                  float64  `json:"x"`
	Y                  float64  `json:"y"`
	Width              float64  `json:"width"`
	Height             float64  `json:"height"`
	OwnerParticipantID *string  `json:"ownerParticipantId"`
	Visibility         string   `json:"visibility"`
	// How many to make. Absent means one, which is what every caller but
	// the new-token dialog sends.
	Count int `json:"count"`
	// Normally absent — a token is created blank and its trackers filled
	// in afterwards. They're here for the same reason TokenID is: undoing
	// a deletion rebuilds the row from this payload alone, and a token
	// that came back on full health would be a worse bug than the misclick
	// the undo was for.
	Trackers   []trackerRequest `json:"trackers"`
	Conditions []string         `json:"conditions"`
}

// handleTokenCreate puts one or more tokens on a scene.
//
// Open to Players as well as GMs, which it wasn't: a Player's summons,
// familiars and companions were the GM's paperwork mid-fight, landing on
// the one person with the least spare attention. What a Player may set
// is narrower, in the same shape token.update already uses — the fields
// that aren't theirs are **ignored rather than rejected**:
//
//   - the owner is the creator. Not a choice, so not a refusal either;
//     this is also the first thing that makes ownership mean something
//     at creation rather than only on a GM's edit.
//   - the token is visible. Hiding something from the room is a GM
//     power, and it's the one field a Player could use to hide a token
//     from the GM.
func (h *Hub) handleTokenCreate(ctx context.Context, c *client, raw json.RawMessage) {
	isGM := c.participant.Role == store.RoleGM

	var req tokenCreateRequest
	if err := decodePayload(raw, &req); err != nil || req.SceneID == "" || req.Name == "" {
		h.sendError(ctx, c, "invalid token.create payload")
		return
	}

	count := req.Count
	if count == 0 {
		count = 1
	}
	if count < 1 || count > maxTokensPerCreate {
		h.sendError(ctx, c, fmt.Sprintf("you can create between 1 and %d tokens at once", maxTokensPerCreate))
		return
	}
	// One id per token or none at all. A short list would leave the
	// client holding ids for tokens that came back under different ones,
	// which is worse than refusing: its undo entries would point at
	// nothing.
	if len(req.TokenIDs) > 0 && len(req.TokenIDs) != count {
		h.sendError(ctx, c, "tokenIds must carry one id per token")
		return
	}
	// Same canonical-form check draw.create makes on a client-chosen id,
	// and for the same reason: the id is echoed back, so anything but the
	// lowercase hyphenated spelling would come back as a different string
	// from the one the client is holding.
	for _, id := range req.TokenIDs {
		if !isCanonicalUUID(id) {
			h.sendError(ctx, c, "tokenId must be a canonical UUID")
			return
		}
	}
	if !h.requireSceneInRoom(ctx, c, req.SceneID) {
		return
	}
	if !h.requireAssetInRoom(ctx, c, req.ImageAssetID) {
		return
	}

	owner := req.OwnerParticipantID
	visibility := store.Visibility(req.Visibility)
	if isGM {
		if !h.requireOwnerInRoom(ctx, c, owner) {
			return
		}
		if visibility == "" {
			visibility = store.VisibilityVisible
		}
		if visibility != store.VisibilityVisible && visibility != store.VisibilityHidden {
			h.sendError(ctx, c, "visibility must be \"visible\" or \"hidden\"")
			return
		}
	} else {
		// Taken from the connection rather than the payload, like every
		// other identity in this file. requireOwnerInRoom is skipped rather
		// than passed: the sender is in the room by construction.
		creator := c.participant.ID
		owner = &creator
		visibility = store.VisibilityVisible
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

	// Only a batch needs to know what's already standing where. A single
	// token goes exactly where it was asked to go — see spawnCells.
	var existing []store.Token
	if count > 1 {
		existing, err = h.store.ListTokensForScene(req.SceneID)
		if err != nil {
			slog.Error("ws: list tokens for spawn failed", "error", err)
			h.sendError(ctx, c, "failed to create token")
			return
		}
	}
	cells := spawnCells(req.X, req.Y, count, width, height, existing)

	created := make([]store.Token, 0, count)
	for i := range count {
		id := ""
		if len(req.TokenIDs) > 0 {
			id = req.TokenIDs[i]
		}
		// No suffix unless there's more than one, or every single token a
		// GM makes picks up a pointless " 1".
		name := req.Name
		if count > 1 {
			name = fmt.Sprintf("%s %d", req.Name, i+1)
		}

		token, err := h.store.CreateToken(store.Token{
			ID:                 id,
			SceneID:            req.SceneID,
			Name:               name,
			ImageAssetID:       req.ImageAssetID,
			X:                  cells[i].X,
			Y:                  cells[i].Y,
			Width:              width,
			Height:             height,
			OwnerParticipantID: owner,
			Visibility:         visibility,
			Trackers:           trackers,
			Conditions:         conditions,
		})
		if err != nil {
			// Whatever was made before the failure is real and stays: the
			// rows exist, so the room has to be told about them. Reporting
			// the failure *and* broadcasting the successes is the honest
			// answer to a half-made batch.
			slog.Error("ws: create token failed", "error", err)
			h.sendError(ctx, c, "failed to create token")
			break
		}
		created = append(created, token)
	}

	// One event per token rather than a batch event: hidden tokens are
	// withheld per recipient, and every client already folds these in one
	// at a time.
	for _, token := range created {
		payload := tokenPayload(token)
		h.broadcastPerClient(ctx, c.roomID, "token.created", func(recipient *client) any {
			if token.Visibility == store.VisibilityHidden && recipient.participant.Role != store.RoleGM {
				return nil
			}
			return payload
		})
	}
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

	// An initiative entry standing for this token shows the token's name
	// and follows the token's visibility, so an edit can change the
	// tracker without anyone having touched it.
	h.broadcastInitiativeIfLinked(ctx, c.roomID, token.ID)
}

type tokenDeleteRequest struct {
	TokenID string `json:"tokenId"`
}

// handleTokenDelete takes a token off the map for everyone: a GM on
// anything, and anyone else on a token they own.
//
// It was GM-only, explicitly because creating one was. Now that a Player
// can make their own, leaving deletion behind would mean eight conjured
// monkeys become the GM's cleanup — which is the busywork the whole
// feature exists to remove. So the rule is ownership, which a token now
// really has: not the erase-style "your own work" rule draw.delete uses,
// since a token has no author, but the same owner already governing its
// trackers and conditions in handleTokenUpdate.
//
// Deletion is still a bigger thing than editing, and the two refusals
// below are worded exactly as that handler's are, for the same reasons:
// a hidden token is refused in the words of one that isn't there,
// including to its own owner, so a GM's ambush stays an ambush.
func (h *Hub) handleTokenDelete(ctx context.Context, c *client, raw json.RawMessage) {
	isGM := c.participant.Role == store.RoleGM

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

	if !isGM {
		if token.Visibility == store.VisibilityHidden {
			h.sendError(ctx, c, "token not found")
			return
		}
		if token.OwnerParticipantID == nil || *token.OwnerParticipantID != c.participant.ID {
			h.sendError(ctx, c, "you can only delete a token you own")
			return
		}
	}

	// Asked before the deletion, not after: the entry is ON DELETE CASCADE
	// from the token, so afterwards there is nothing left to notice.
	wasInInitiative := h.tokenIsInInitiative(c.roomID, token.ID)

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

	// The entry went with it, so the room has to be told the order is one
	// shorter — and if it was that combatant's turn, that nobody's is now.
	if wasInInitiative {
		h.broadcastInitiative(ctx, c.roomID)
	}
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

// The cap on req.Cells reuses maxPaintedCells: a hand-swept drag was
// self-limiting (bounded by how many pointer-move events a human drag
// produces), but the rectangle fog tool sends every cell in its
// bounding box in one command, so a corner-to-corner drag across a
// large map arrives as one very long list.
func (h *Hub) handleFogReveal(ctx context.Context, c *client, raw json.RawMessage) {
	h.handlePaintFog(ctx, c, raw, "fog.reveal", "fog.revealed", h.store.RevealCells)
}

// handleFogHide puts revealed cells back under the cover, so a reveal
// painted over the wrong corridor can be taken back without resetting
// the scene. GM-only for the same reason revealing is: fog is the GM's
// control over what the room is allowed to see.
func (h *Hub) handleFogHide(ctx context.Context, c *client, raw json.RawMessage) {
	h.handlePaintFog(ctx, c, raw, "fog.hide", "fog.hidden", h.store.HideCells)
}

// handlePaintFog is the whole of both painting commands: they take the
// same payload, differ only in which store call they make, and both
// broadcast the chunks that call reports as changed. Neither has to
// care which direction it's painting in beyond that.
func (h *Hub) handlePaintFog(
	ctx context.Context,
	c *client,
	raw json.RawMessage,
	command, event string,
	paint func(string, []store.FogCell) ([]store.FogChunk, error),
) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req fogCellsRequest
	if err := decodePayload(raw, &req); err != nil || req.SceneID == "" || len(req.Cells) == 0 {
		h.sendError(ctx, c, "invalid "+command+" payload")
		return
	}
	if len(req.Cells) > maxPaintedCells {
		h.sendError(ctx, c, fmt.Sprintf("that's %d cells, too many to paint at once", len(req.Cells)))
		return
	}
	if !h.requireSceneInRoom(ctx, c, req.SceneID) {
		return
	}

	changed, err := paint(req.SceneID, req.Cells)
	if err != nil {
		slog.Error("ws: paint fog failed", "command", command, "error", err)
		h.sendError(ctx, c, "failed to change fog")
		return
	}
	// A drag over cells already in the target state changes no chunk and
	// so says nothing — both commands are idempotent, and an event
	// carrying no chunks would be one every client does nothing with.
	if len(changed) == 0 {
		return
	}

	h.broadcast(ctx, c.roomID, event, map[string]any{
		"sceneId": req.SceneID,
		"chunks":  changed,
	})
}

// handleFogRevealAll uncovers the whole scene at once — for a map that
// doesn't want fog, or the moment an encounter ends.
//
// It needs neither the scene's bounds nor a cap, because it only has to
// describe the chunks that currently hold fog rather than every cell a
// map could have. That is the direct payoff of storing what's hidden:
// this used to be the one operation big enough to need limiting, and
// the limit has moved to handleFogReset below.
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

	cleared, err := h.store.RevealAllCells(req.SceneID)
	if err != nil {
		slog.Error("ws: reveal all fog failed", "error", err)
		h.sendError(ctx, c, "failed to reveal fog")
		return
	}
	if len(cleared) == 0 {
		return // already had no fog on it
	}

	// Deliberately the same event a painted reveal broadcasts, so clients
	// need no separate case for it: chunks zeroed here merge exactly like
	// chunks zeroed by a drag.
	h.broadcast(ctx, c.roomID, "fog.revealed", map[string]any{
		"sceneId": req.SceneID,
		"chunks":  cleared,
	})
}

// handleFogReset covers a whole scene — the classic dungeon-crawl
// starting point, and the opposite of the state a scene is created in.
// There's no undo for this; see the note on the toolbar button.
//
// This is the operation that has to enumerate a scene's bounds, and so
// the one carrying the cap. Reveal-all used to be that operation; the
// two swapped places when fog started storing what's hidden.
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

	scene, err := h.store.GetScene(req.SceneID)
	if err != nil {
		slog.Error("ws: load scene for fog reset failed", "error", err)
		h.sendError(ctx, c, "failed to reset fog")
		return
	}

	chunks, err := sceneFogChunks(scene)
	if err != nil {
		// Bounds the GM can see and fix, not an internal failure, so the
		// reason goes to them verbatim.
		h.sendError(ctx, c, err.Error())
		return
	}

	delta, err := h.store.HideAllCells(req.SceneID, chunks)
	if err != nil {
		slog.Error("ws: reset fog failed", "error", err)
		h.sendError(ctx, c, "failed to reset fog")
		return
	}

	// fog.hidden rather than an event of its own — covering everything is
	// covering every chunk, which is what a painted hide already says.
	// The delta includes any chunk outside these bounds zeroed, so a
	// client can't be left drawing fog this just deleted.
	h.broadcast(ctx, c.roomID, "fog.hidden", map[string]any{
		"sceneId": req.SceneID,
		"chunks":  delta,
	})
}

// maxPaintedCells caps how many cells one fog.reveal or fog.hide may
// name. The rectangle tool sends every cell in its bounding box, so a
// corner-to-corner drag on a large map is a long list — but it is a
// list of cells the *client* chose, so unlike the chunk cap below this
// bounds a payload rather than a map. 40,000 is a 200x200 grid.
const maxPaintedCells = 40_000

// maxFogChunks caps what one fog.reset may materialise. Every chunk is
// both a row inserted and an entry in the payload every client
// receives, and the count grows with the product of the map's
// dimensions in grid squares. 2,000 chunks is a little over a 200x200
// grid (200 rows of 7 chunks each) — far past any map anyone actually
// plays on, and 32x more cells than the same number of rows used to buy
// when fog was stored a cell at a time.
const maxFogChunks = 2_000

// sceneFogChunks enumerates every chunk inside a scene's bounds, fully
// hidden, indexed the way the client indexes a painted one —
// floor(pixel / gridSize) from the origin — so a covered-everything
// scene and a hand-painted one agree on what a cell is.
func sceneFogChunks(scene store.Scene) ([]store.FogChunk, error) {
	if scene.GridSize <= 0 || scene.Width <= 0 || scene.Height <= 0 {
		return nil, errors.New("this scene has no map bounds to cover")
	}

	// Rounded up, so a map whose last row or column of squares is
	// clipped still gets covered rather than leaving a revealed strip.
	cols := (scene.Width + scene.GridSize - 1) / scene.GridSize
	rows := (scene.Height + scene.GridSize - 1) / scene.GridSize
	chunksPerRow := (cols + store.FogChunkWidth - 1) / store.FogChunkWidth
	if rows*chunksPerRow > maxFogChunks {
		return nil, fmt.Errorf("this scene covers %d cells, too many to cover at once", cols*rows)
	}

	chunks := make([]store.FogChunk, 0, rows*chunksPerRow)
	for y := range rows {
		for cx := range chunksPerRow {
			// The last chunk of a row is partial unless the map happens to
			// be a multiple of 32 cells wide. Setting only the bits that
			// are really there keeps fog inside the map rather than
			// hanging up to 31 covered cells off its right edge.
			mask := ^uint32(0)
			if remaining := cols - cx*store.FogChunkWidth; remaining < store.FogChunkWidth {
				mask = 1<<uint(remaining) - 1
			}
			chunks = append(chunks, store.FogChunk{Y: y, ChunkX: cx, Mask: mask})
		}
	}
	return chunks, nil
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

// Stroke width in world pixels. The default matches DRAWING_STROKE_WIDTH
// in web/src/lib/drawing-hit.ts and is what every drawing was rendered at
// before the width was per-stroke.
//
// The bounds are a sanity clamp rather than the control's range: a width
// is world pixels on a shared map, so an absurd one isn't just the
// sender's problem — a 10,000px stroke is an opaque sheet over everyone
// else's scene, and there is no undo for someone else's drawing unless
// you are the GM.
const (
	defaultDrawingStrokeWidth = 3
	minDrawingStrokeWidth     = 1
	maxDrawingStrokeWidth     = 32
)

type drawCreateRequest struct {
	// DrawingID is chosen by the client, which has already drawn the
	// stroke under it rather than waiting for this round trip. Optional:
	// a client that doesn't care gets a server-generated one.
	DrawingID string        `json:"drawingId"`
	SceneID   string        `json:"sceneId"`
	Kind      string        `json:"kind"`
	Points    []store.Point `json:"points"`
	Color     string        `json:"color"`
	Filled    bool          `json:"filled"`
	// Zero means "not specified", which is what an older client sends by
	// omitting the field — so it takes the default rather than a hairline.
	StrokeWidth float64 `json:"strokeWidth"`
}

// canFill reports whether a kind encloses an area worth shading. A line
// and a freehand stroke don't, and a filled one of either is not a
// drawing anybody can render — Konva would close the path and shade
// whatever the stroke happened to loop around.
func canFill(kind store.DrawingKind) bool {
	return kind == store.DrawingKindRect || kind == store.DrawingKindEllipse
}

// strokeWidthOrDefault clamps rather than rejects, for the same reason
// the fill flag is normalised: the client asked for a drawing, and the
// useful answer is the drawing it meant. A width outside the bounds is
// the one case that would otherwise reach every other person's map.
func strokeWidthOrDefault(width float64) float64 {
	if width <= 0 {
		return defaultDrawingStrokeWidth
	}
	return min(max(width, minDrawingStrokeWidth), maxDrawingStrokeWidth)
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
		ID:      req.DrawingID,
		SceneID: req.SceneID,
		Kind:    kind,
		Points:  req.Points,
		Color:   color,
		// Normalised rather than refused. A fill on a line is a client
		// sending a flag that means nothing for the kind it picked, not an
		// attempt at anything — the useful answer is the drawing they meant,
		// not an error naming a field they may not know they sent.
		Filled:                 req.Filled && canFill(kind),
		StrokeWidth:            strokeWidthOrDefault(req.StrokeWidth),
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
		"sceneId": req.SceneID,
		"x":       req.X,
		"y":       req.Y,
		// The id as well as the name, so a recipient can look the pinger's
		// identity colour up in the roster it already has. The colour
		// itself deliberately doesn't ride along: it would be a second
		// copy of something the roster already answers, going stale the
		// moment a seat's colour changes.
		"participantId":   c.participant.ID,
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
// announceDeparture tells the room that someone's grace period ran out.
// Its own context, because the connection whose closing started all this
// was torn down a grace period ago and took its context with it.
func (h *Hub) announceDeparture(roomID string, participant store.Participant) {
	ctx, cancel := context.WithTimeout(context.Background(), measureCleanupTimeout)
	defer cancel()
	h.broadcast(ctx, roomID, "participant.disconnected", map[string]any{
		"participantId": participant.ID,
	})
	h.postSystemMessage(ctx, roomID, participant, store.SystemEventLeft)
}

// postSystemMessage writes a line the room said about itself, rather
// than one a person said, and broadcasts it down the same `chat.posted`
// event as any other message — so a client that already renders chat
// history and hydrates it from state.sync needs no new plumbing.
//
// It takes a room rather than a client, and reports failure to nobody:
// there is often no connection left to tell. A departure in particular
// is announced by a timer whose participant has been gone half a minute,
// so a failed insert is the server's problem alone and belongs in the
// log with the rest of them.
func (h *Hub) postSystemMessage(ctx context.Context, roomID string, participant store.Participant, event store.SystemEvent) {
	msg, err := h.store.InsertMessage(store.Message{
		RoomID: roomID,
		// No participant id, deliberately, though the name is kept. Two
		// reasons, and the first one bit: a GM removing a seat while that
		// person is inside their grace window deletes the row this would
		// point at, and the foreign key then refuses the very line saying
		// they left. The second is that nobody wrote this message —
		// pointing it at a participant would make it theirs, and "theirs"
		// is what chat.delete checks when deciding who may remove it.
		ParticipantID:   nil,
		ParticipantName: participant.DisplayName,
		Kind:            store.MessageKindSystem,
		// The event, never the sentence — see the Body comment on
		// store.Message. The client writes the words.
		Body: string(event),
	})
	if err != nil {
		slog.Error("ws: insert system message failed", "error", err, "event", event)
		return
	}
	h.broadcast(ctx, roomID, "chat.posted", messagePayload(msg, ""))
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

	// Nothing to do about fog here. A new scene is fully revealed because
	// fog stores what's *hidden* and a new scene has none — it isn't a
	// default anyone has to apply. This used to materialise a revealed
	// cell for every square in bounds, capped, and silently give up on
	// maps too large for that cap.

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
