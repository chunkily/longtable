package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"longtable/internal/store"
)

// The initiative tracker: six commands and one event, in their own file
// rather than in hub.go, which is long enough that the token handlers
// are already hard to find. Everything here follows the same shape as
// its neighbours there — decode, GM check, apply, broadcast.
//
// **Every change broadcasts the whole tracker**, not a delta. Three
// reasons, in order of how much they cost to work around: entries are
// withheld per recipient (a Player must not learn that a hidden
// combatant exists), a turn advance can change the round as well as the
// pointer, and a removal can move whose turn it is — so most "small"
// changes are several fields anyway. A tracker is a couple of dozen
// entries at most, and it changes once a turn.

const maxInitiativeName = 40

// initiativeItem is an entry with the token bits already resolved, so
// the per-recipient filtering below is pure in-memory work rather than
// a database read per client.
type initiativeItem struct {
	entry        store.InitiativeEntry
	name         string
	imageAssetID *string
	// True when this entry stands for a token Players can't see. Kept
	// apart from the entry's own Hidden flag because they mean different
	// things: this one follows the token and can change without anyone
	// touching the tracker.
	tokenHidden bool
}

// initiativeItems reads the tracker and resolves each linked entry
// against its token — name and art come from the token every time
// rather than being copied at creation, so renaming a token renames its
// entry for everyone.
func (h *Hub) initiativeItems(roomID string) ([]initiativeItem, store.InitiativeState, error) {
	state, err := h.store.GetInitiativeState(roomID)
	if err != nil {
		return nil, store.InitiativeState{}, err
	}

	items := make([]initiativeItem, 0, len(state.Entries))
	for _, entry := range state.Entries {
		item := initiativeItem{entry: entry, name: entry.Name}
		if entry.TokenID != nil {
			token, err := h.store.GetToken(*entry.TokenID)
			if err != nil {
				// The row is gone but the cascade hasn't been seen here yet,
				// or something worse. Either way the entry has no name to
				// show, so it is left out rather than drawn as a blank.
				slog.Error("ws: initiative entry token missing", "error", err)
				continue
			}
			item.name = token.Name
			item.imageAssetID = token.ImageAssetID
			item.tokenHidden = token.Visibility == store.VisibilityHidden
		}
		items = append(items, item)
	}
	return items, state, nil
}

// initiativePayload builds the tracker as one role may see it.
//
// A Player is shown neither an entry the GM marked hidden nor one
// standing for a hidden token — the second is the same rule the token
// itself obeys, answered from the token so the two can't disagree.
// `currentEntryId` is still sent when it names a withheld entry: the
// Player then sees a tracker with nothing highlighted, which says "it's
// somebody's turn and not yours" without saying whose.
func initiativePayload(items []initiativeItem, state store.InitiativeState, role store.Role) map[string]any {
	entries := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if role != store.RoleGM && (item.entry.Hidden || item.tokenHidden) {
			continue
		}
		entries = append(entries, map[string]any{
			"id":           item.entry.ID,
			"tokenId":      item.entry.TokenID,
			"name":         item.name,
			"initiative":   item.entry.Initiative,
			"hidden":       item.entry.Hidden || item.tokenHidden,
			"imageAssetId": item.imageAssetID,
		})
	}

	return map[string]any{
		"entries":        entries,
		"round":          state.Round,
		"currentEntryId": state.CurrentEntryID,
	}
}

// initiativeStatePayload is what state.sync carries, for one client.
func (h *Hub) initiativeStatePayload(roomID string, role store.Role) map[string]any {
	items, state, err := h.initiativeItems(roomID)
	if err != nil {
		slog.Error("ws: read initiative failed", "error", err)
		return map[string]any{"entries": []any{}, "round": 1, "currentEntryId": nil}
	}
	return initiativePayload(items, state, role)
}

// broadcastInitiative sends the tracker to the whole room, filtered per
// recipient. Read once here rather than once per client.
func (h *Hub) broadcastInitiative(ctx context.Context, roomID string) {
	items, state, err := h.initiativeItems(roomID)
	if err != nil {
		slog.Error("ws: read initiative failed", "error", err)
		return
	}
	h.broadcastPerClient(ctx, roomID, "initiative.updated", func(recipient *client) any {
		return initiativePayload(items, state, recipient.participant.Role)
	})
}

// broadcastInitiativeIfLinked re-sends the tracker when something has
// happened to a token an entry stands for.
//
// Without it a linked entry goes stale in three ways that all look like
// bugs: a renamed token keeps its old name in the order, a token hidden
// mid-fight stays listed for Players, and a revealed one never appears.
// The tracker takes its name and visibility from the token, so anything
// that changes a token can change the tracker.
func (h *Hub) broadcastInitiativeIfLinked(ctx context.Context, roomID, tokenID string) {
	if h.tokenIsInInitiative(roomID, tokenID) {
		h.broadcastInitiative(ctx, roomID)
	}
}

// tokenIsInInitiative reports whether any entry stands for this token.
//
// Its awkward existence is a deletion problem: `initiative_entry.token_id`
// is ON DELETE CASCADE, so by the time a token has been deleted there is
// no entry left to notice, and asking afterwards always answers no. The
// deletion path has to ask *before* it deletes and broadcast after.
func (h *Hub) tokenIsInInitiative(roomID, tokenID string) bool {
	entries, err := h.store.ListInitiativeEntries(roomID)
	if err != nil {
		slog.Error("ws: read initiative failed", "error", err)
		return false
	}
	for _, entry := range entries {
		if entry.TokenID != nil && *entry.TokenID == tokenID {
			return true
		}
	}
	return false
}

type initiativeAddRequest struct {
	// Exactly one of these two says what the entry is: a token to stand
	// for, or a name to stand alone under.
	TokenID    *string `json:"tokenId"`
	Name       string  `json:"name"`
	Initiative float64 `json:"initiative"`
	Hidden     bool    `json:"hidden"`
}

func (h *Hub) handleInitiativeAdd(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req initiativeAddRequest
	if err := decodePayload(raw, &req); err != nil {
		h.sendError(ctx, c, "invalid initiative.add payload")
		return
	}

	name := strings.TrimSpace(req.Name)
	if req.TokenID == nil && name == "" {
		h.sendError(ctx, c, "an entry needs a token or a name")
		return
	}
	if len([]rune(name)) > maxInitiativeName {
		h.sendError(ctx, c, "that name is too long")
		return
	}

	if req.TokenID != nil {
		// Scoped through the token's own scene, like every other command
		// that takes a token id: a tracker is not a way to find out what
		// exists in someone else's room.
		token, err := h.store.GetToken(*req.TokenID)
		if err != nil || !h.sceneInRoom(c, token.SceneID) {
			h.sendError(ctx, c, "token not found")
			return
		}
		// The name is the token's from here on, so whatever was typed is
		// dropped rather than stored and ignored.
		name = ""
	}

	if _, err := h.store.CreateInitiativeEntry(store.InitiativeEntry{
		RoomID:     c.roomID,
		TokenID:    req.TokenID,
		Name:       name,
		Initiative: req.Initiative,
		// Only meaningful for a freestanding entry; a linked one takes its
		// visibility from the token.
		Hidden: req.TokenID == nil && req.Hidden,
	}); err != nil {
		slog.Error("ws: create initiative entry failed", "error", err)
		h.sendError(ctx, c, "failed to add that entry")
		return
	}

	h.broadcastInitiative(ctx, c.roomID)
}

type initiativeEntryRequest struct {
	EntryID string `json:"entryId"`
}

type initiativeUpdateRequest struct {
	EntryID    string  `json:"entryId"`
	Name       string  `json:"name"`
	Initiative float64 `json:"initiative"`
	Hidden     bool    `json:"hidden"`
}

// handleInitiativeUpdate edits an entry's value, name and hidden flag.
// Every field every time, like token.update: the wire can't tell "left
// alone" from "cleared", and a name that came back empty would leave a
// nameless combatant in the order.
func (h *Hub) handleInitiativeUpdate(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req initiativeUpdateRequest
	if err := decodePayload(raw, &req); err != nil || req.EntryID == "" {
		h.sendError(ctx, c, "invalid initiative.update payload")
		return
	}

	entry, ok := h.initiativeEntryInRoom(ctx, c, req.EntryID)
	if !ok {
		return
	}

	name := strings.TrimSpace(req.Name)
	if len([]rune(name)) > maxInitiativeName {
		h.sendError(ctx, c, "that name is too long")
		return
	}
	// A linked entry's name and visibility belong to its token, so the
	// two fields are ignored here rather than rejected — the same
	// treatment token.update gives a field the sender may not set.
	if entry.TokenID == nil {
		if name == "" {
			h.sendError(ctx, c, "an entry needs a name")
			return
		}
		entry.Name = name
		entry.Hidden = req.Hidden
	}
	entry.Initiative = req.Initiative

	if err := h.store.UpdateInitiativeEntry(entry); err != nil {
		slog.Error("ws: update initiative entry failed", "error", err)
		h.sendError(ctx, c, "failed to save that entry")
		return
	}

	h.broadcastInitiative(ctx, c.roomID)
}

// handleInitiativeRemove takes a combatant out of the order, and hands
// the turn on if it was theirs — a tracker stuck on a creature that has
// just died would need the GM to notice and click twice.
func (h *Hub) handleInitiativeRemove(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req initiativeEntryRequest
	if err := decodePayload(raw, &req); err != nil || req.EntryID == "" {
		h.sendError(ctx, c, "invalid initiative.remove payload")
		return
	}
	if _, ok := h.initiativeEntryInRoom(ctx, c, req.EntryID); !ok {
		return
	}

	state, err := h.store.GetInitiativeState(c.roomID)
	if err != nil {
		slog.Error("ws: read initiative failed", "error", err)
		h.sendError(ctx, c, "failed to remove that entry")
		return
	}

	// Worked out before the deletion, while the order still holds the
	// entry being removed — afterwards there is nothing to be "next" of.
	if state.CurrentEntryID != nil && *state.CurrentEntryID == req.EntryID {
		next := nextEntryAfter(state.Entries, req.EntryID)
		if err := h.store.SetInitiativeTurn(c.roomID, next, state.Round); err != nil {
			slog.Error("ws: hand on initiative turn failed", "error", err)
			h.sendError(ctx, c, "failed to remove that entry")
			return
		}
	}

	if err := h.store.DeleteInitiativeEntry(req.EntryID); err != nil {
		slog.Error("ws: delete initiative entry failed", "error", err)
		h.sendError(ctx, c, "failed to remove that entry")
		return
	}

	h.broadcastInitiative(ctx, c.roomID)
}

// nextEntryAfter is who takes the turn when `id` leaves the order,
// wrapping to the top. Nil when `id` was the only entry.
func nextEntryAfter(entries []store.InitiativeEntry, id string) *string {
	for i, e := range entries {
		if e.ID != id {
			continue
		}
		if len(entries) == 1 {
			return nil
		}
		next := entries[(i+1)%len(entries)]
		return &next.ID
	}
	return nil
}

type initiativeReorderRequest struct {
	EntryID   string `json:"entryId"`
	Direction string `json:"direction"`
}

// handleInitiativeReorder moves an entry one place up or down among the
// combatants it is tied with.
//
// Only among ties, and that is the whole feature rather than a
// limitation: the order *is* the initiative values, and letting an
// entry jump above a higher roll would make the list disagree with the
// numbers printed beside it. Tables settle ties by argument, so ties are
// the case that needs a manual answer.
func (h *Hub) handleInitiativeReorder(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req initiativeReorderRequest
	if err := decodePayload(raw, &req); err != nil || req.EntryID == "" {
		h.sendError(ctx, c, "invalid initiative.reorder payload")
		return
	}
	step := 0
	switch req.Direction {
	case "up":
		step = -1
	case "down":
		step = 1
	default:
		h.sendError(ctx, c, `direction must be "up" or "down"`)
		return
	}

	entries, err := h.store.ListInitiativeEntries(c.roomID)
	if err != nil {
		slog.Error("ws: read initiative failed", "error", err)
		h.sendError(ctx, c, "failed to reorder that entry")
		return
	}

	index := -1
	for i, e := range entries {
		if e.ID == req.EntryID {
			index = i
			break
		}
	}
	if index == -1 {
		h.sendError(ctx, c, "entry not found")
		return
	}
	target := index + step
	if target < 0 || target >= len(entries) {
		h.sendError(ctx, c, "that entry is already at the end of the order")
		return
	}
	if entries[target].Initiative != entries[index].Initiative {
		h.sendError(ctx, c, "change the initiative value to move past a different roll")
		return
	}

	entries[index], entries[target] = entries[target], entries[index]
	// Renumbered from the whole list rather than by swapping two values:
	// every new entry starts at sort_order 0, so swapping would trade one
	// zero for another and change nothing at all.
	for i := range entries {
		entries[i].SortOrder = i
		if err := h.store.UpdateInitiativeEntry(entries[i]); err != nil {
			slog.Error("ws: reorder initiative failed", "error", err)
			h.sendError(ctx, c, "failed to reorder that entry")
			return
		}
	}

	h.broadcastInitiative(ctx, c.roomID)
}

type initiativeAdvanceRequest struct {
	Direction string `json:"direction"`
}

// handleInitiativeAdvance moves the turn on (or back), carrying the
// round with it.
//
// The round changes only at the wrap, in both directions, which is what
// makes "next then previous" land exactly where it started — including
// across a round boundary, where an off-by-one leaves a table arguing
// about whether a spell has expired.
func (h *Hub) handleInitiativeAdvance(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}

	var req initiativeAdvanceRequest
	if err := decodePayload(raw, &req); err != nil {
		h.sendError(ctx, c, "invalid initiative.advance payload")
		return
	}
	if req.Direction != "next" && req.Direction != "previous" {
		h.sendError(ctx, c, `direction must be "next" or "previous"`)
		return
	}

	state, err := h.store.GetInitiativeState(c.roomID)
	if err != nil {
		slog.Error("ws: read initiative failed", "error", err)
		h.sendError(ctx, c, "failed to change the turn")
		return
	}
	if len(state.Entries) == 0 {
		h.sendError(ctx, c, "there is nobody in the order yet")
		return
	}

	next, round := advanceTurn(state, req.Direction)
	if err := h.store.SetInitiativeTurn(c.roomID, &next, round); err != nil {
		slog.Error("ws: set initiative turn failed", "error", err)
		h.sendError(ctx, c, "failed to change the turn")
		return
	}

	h.broadcastInitiative(ctx, c.roomID)
}

// advanceTurn is the turn arithmetic, kept out of the handler because
// it is the part with edge cases: the first press of Next with nobody
// up yet, the wrap in each direction, and a round counter that must
// never read zero.
func advanceTurn(state store.InitiativeState, direction string) (string, int) {
	// Nobody is up yet: the first press starts the encounter at the top
	// of the order rather than counting a round nobody has played.
	if state.CurrentEntryID == nil {
		if direction == "next" {
			return state.Entries[0].ID, state.Round
		}
		return state.Entries[len(state.Entries)-1].ID, state.Round
	}

	index := 0
	for i, e := range state.Entries {
		if e.ID == *state.CurrentEntryID {
			index = i
			break
		}
	}

	round := state.Round
	if direction == "next" {
		if index == len(state.Entries)-1 {
			round++
		}
		return state.Entries[(index+1)%len(state.Entries)].ID, round
	}

	if index == 0 {
		// Round 1 is the floor: going back before the first turn of the
		// first round would be a fight that hasn't happened.
		if round > 1 {
			round--
		}
		return state.Entries[len(state.Entries)-1].ID, round
	}
	return state.Entries[index-1].ID, round
}

// handleInitiativeClear empties the tracker for the next encounter. The
// tokens themselves are untouched: leaving the order is not leaving the
// map.
func (h *Hub) handleInitiativeClear(ctx context.Context, c *client) {
	if !h.requireGM(ctx, c) {
		return
	}

	if err := h.store.ClearInitiative(c.roomID); err != nil {
		slog.Error("ws: clear initiative failed", "error", err)
		h.sendError(ctx, c, "failed to clear the tracker")
		return
	}

	h.broadcastInitiative(ctx, c.roomID)
}

// initiativeEntryInRoom resolves an entry id and checks it belongs to
// this room, reporting the refusal itself. An entry from another room
// answers exactly as one that doesn't exist.
func (h *Hub) initiativeEntryInRoom(
	ctx context.Context, c *client, entryID string,
) (store.InitiativeEntry, bool) {
	entry, err := h.store.GetInitiativeEntry(entryID)
	if err != nil || entry.RoomID != c.roomID {
		h.sendError(ctx, c, "entry not found")
		return store.InitiativeEntry{}, false
	}
	return entry, true
}
