package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"longtable/internal/store"
)

// The party: the characters a GM puts on a new scene in one go, since
// every token belongs to exactly one scene.
//
// **A member is defined, not marked.** The first version made the party a
// mark on tokens, with the newest copy of each marked character standing
// in for it. It worked, and it put a character's details on whichever map
// it had last been edited on. Now the GM defines each member once in the
// Party dialog — name, art, size, owner — and tokens on maps are copies
// made from it, each carrying the member's id as its party key.
//
// Everything here is GM-only, and so is the roster itself: party.updated
// goes to GMs alone and a Player's state.sync has no party in it. The
// copies, once down, are tokens like any other and everyone sees them.
//
// **Every change broadcasts the whole roster**, the way initiative does:
// it's a handful of rows, and a client folding in one shape is simpler
// than three.

// maxPartySize bounds a party for the reason maxTokensPerCreate bounds a
// batch: party.add makes every member at once, and each is a row, a
// broadcast to every client and a group the canvas rebuilds.
const maxPartySize = maxTokensPerCreate

type partyMemberRequest struct {
	// Only on an update. A create's id is the server's.
	MemberID           string  `json:"memberId"`
	Name               string  `json:"name"`
	ImageAssetID       *string `json:"imageAssetId"`
	Width              float64 `json:"width"`
	Height             float64 `json:"height"`
	OwnerParticipantID *string `json:"ownerParticipantId"`
	// Absent means attending — what a new member is, and what a client
	// that predates the checkbox would have meant.
	Attending *bool `json:"attending"`
	// What its copies wear when it has no art, as on a token.
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

func (r partyMemberRequest) attending() bool {
	return r.Attending == nil || *r.Attending
}

func partyMemberPayload(m store.PartyMember) map[string]any {
	return map[string]any{
		"id":                 m.ID,
		"name":               m.Name,
		"imageAssetId":       m.ImageAssetID,
		"width":              m.Width,
		"height":             m.Height,
		"ownerParticipantId": m.OwnerParticipantID,
		"attending":          m.Attending,
		"icon":               m.Icon,
		"color":              m.Color,
	}
}

func (h *Hub) partyPayload(roomID string) ([]map[string]any, error) {
	members, err := h.store.ListPartyMembers(roomID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, len(members))
	for i, m := range members {
		out[i] = partyMemberPayload(m)
	}
	return out, nil
}

// broadcastParty sends the whole roster to every GM in the room.
func (h *Hub) broadcastParty(ctx context.Context, roomID string) {
	party, err := h.partyPayload(roomID)
	if err != nil {
		slog.Error("ws: list party failed", "error", err)
		return
	}
	payload := map[string]any{"members": party}
	h.broadcastPerClient(ctx, roomID, "party.updated", func(recipient *client) any {
		if recipient.participant.Role != store.RoleGM {
			return nil
		}
		return payload
	})
}

// decodeMember reads and checks a member's details: the same checks a
// GM's token.create makes on the same fields, since a member is what a
// token will be made from.
func (h *Hub) decodeMember(ctx context.Context, c *client, raw json.RawMessage, command string) (partyMemberRequest, bool) {
	var req partyMemberRequest
	if err := decodePayload(raw, &req); err != nil {
		h.sendError(ctx, c, fmt.Sprintf("invalid %s payload", command))
		return req, false
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		h.sendError(ctx, c, "A party member needs a name.")
		return req, false
	}
	if !h.requireAssetInRoom(ctx, c, req.ImageAssetID) {
		return req, false
	}
	if !h.requireOwnerInRoom(ctx, c, req.OwnerParticipantID) {
		return req, false
	}
	if !h.requireIconAndColor(ctx, c, req.Icon, req.Color) {
		return req, false
	}
	if req.Width == 0 {
		req.Width = 1
	}
	if req.Height == 0 {
		req.Height = 1
	}
	return req, true
}

func (h *Hub) handlePartyMemberCreate(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}
	req, ok := h.decodeMember(ctx, c, raw, "party.memberCreate")
	if !ok {
		return
	}
	n, err := h.store.CountPartyMembers(c.roomID)
	if err != nil {
		slog.Error("ws: count party failed", "error", err)
		h.sendError(ctx, c, "failed to add to the party")
		return
	}
	if n >= maxPartySize {
		h.sendError(ctx, c, fmt.Sprintf("A party can have up to %d members.", maxPartySize))
		return
	}
	if _, err := h.store.CreatePartyMember(store.PartyMember{
		RoomID:             c.roomID,
		Name:               req.Name,
		ImageAssetID:       req.ImageAssetID,
		Width:              req.Width,
		Height:             req.Height,
		OwnerParticipantID: req.OwnerParticipantID,
		Attending:          req.attending(),
		Icon:               req.Icon,
		Color:              req.Color,
	}); err != nil {
		slog.Error("ws: create party member failed", "error", err)
		h.sendError(ctx, c, "failed to add to the party")
		return
	}
	h.broadcastParty(ctx, c.roomID)
}

func (h *Hub) handlePartyMemberUpdate(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}
	req, ok := h.decodeMember(ctx, c, raw, "party.memberUpdate")
	if !ok {
		return
	}
	err := h.store.UpdatePartyMember(store.PartyMember{
		ID:                 req.MemberID,
		RoomID:             c.roomID,
		Name:               req.Name,
		ImageAssetID:       req.ImageAssetID,
		Width:              req.Width,
		Height:             req.Height,
		OwnerParticipantID: req.OwnerParticipantID,
		Attending:          req.attending(),
		Icon:               req.Icon,
		Color:              req.Color,
	})
	if errors.Is(err, store.ErrNotFound) {
		h.sendError(ctx, c, "party member not found")
		return
	}
	if err != nil {
		slog.Error("ws: update party member failed", "error", err)
		h.sendError(ctx, c, "failed to update the party")
		return
	}
	h.broadcastParty(ctx, c.roomID)
}

type partyMemberDeleteRequest struct {
	MemberID string `json:"memberId"`
}

// handlePartyMemberDelete takes a character out of the party. Its copies
// stay on their maps as ordinary tokens: leaving the party is not leaving
// the scene, and a GM who means that deletes the token.
func (h *Hub) handlePartyMemberDelete(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}
	var req partyMemberDeleteRequest
	if err := decodePayload(raw, &req); err != nil || req.MemberID == "" {
		h.sendError(ctx, c, "invalid party.memberDelete payload")
		return
	}
	err := h.store.DeletePartyMember(c.roomID, req.MemberID)
	if errors.Is(err, store.ErrNotFound) {
		h.sendError(ctx, c, "party member not found")
		return
	}
	if err != nil {
		slog.Error("ws: delete party member failed", "error", err)
		h.sendError(ctx, c, "failed to update the party")
		return
	}
	h.broadcastParty(ctx, c.roomID)
}

type partyAddRequest struct {
	SceneID string  `json:"sceneId"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
}

// handlePartyAdd puts a copy of every attending member not already on a
// scene onto it, spread out from (x, y) the way a batch from the New token dialog is
// — though by footprint each, since a party mixes sizes.
//
// A copy is the member: name, art, size, owner, icon and colour. It is visible, has
// blank trackers and no conditions, and carries the member's id so a
// second press skips it. It is a copy and not a link — editing the
// member later changes the next copy, not this one.
//
// The sender alone is then told which tokens were made, in party.added,
// so its undo stack can hold one entry per token the way a batch's does.
// It couldn't mint the ids itself as token.create's caller does: it can't
// know how many members are already on the scene.
func (h *Hub) handlePartyAdd(ctx context.Context, c *client, raw json.RawMessage) {
	if !h.requireGM(ctx, c) {
		return
	}
	var req partyAddRequest
	if err := decodePayload(raw, &req); err != nil || req.SceneID == "" {
		h.sendError(ctx, c, "invalid party.add payload")
		return
	}
	if !h.requireSceneInRoom(ctx, c, req.SceneID) {
		return
	}

	members, err := h.store.ListPartyMembers(c.roomID)
	if err != nil {
		slog.Error("ws: list party failed", "error", err)
		h.sendError(ctx, c, "failed to add the party")
		return
	}
	if len(members) == 0 {
		h.sendError(ctx, c, "No one is in the party yet.")
		return
	}
	// Only who's at the table tonight. Nobody ticked is said as its own
	// thing rather than as "already here", which would be a lie.
	var attending []store.PartyMember
	for _, m := range members {
		if m.Attending {
			attending = append(attending, m)
		}
	}
	if len(attending) == 0 {
		h.sendError(ctx, c, "No one in the party is ticked as attending.")
		return
	}
	existing, err := h.store.ListTokensForScene(req.SceneID)
	if err != nil {
		slog.Error("ws: list tokens for party failed", "error", err)
		h.sendError(ctx, c, "failed to add the party")
		return
	}

	// Anyone already here stays where they are and doesn't get a twin —
	// which is also what makes a second press harmless, and lets a member
	// added since be put down by pressing it again.
	here := map[string]bool{}
	for _, t := range existing {
		if t.PartyKey != "" {
			here[t.PartyKey] = true
		}
	}
	var missing []store.PartyMember
	for _, m := range attending {
		if !here[m.ID] {
			missing = append(missing, m)
		}
	}
	if len(missing) == 0 {
		h.sendError(ctx, c, "The whole party is already here.")
		return
	}

	sizes := make([]cell, len(missing))
	for i, m := range missing {
		sizes[i] = cell{X: m.Width, Y: m.Height}
	}
	cells := spawnFootprints(req.X, req.Y, sizes, existing)

	created := make([]store.Token, 0, len(missing))
	for i, m := range missing {
		token, err := h.store.CreateToken(store.Token{
			SceneID:            req.SceneID,
			Name:               m.Name,
			ImageAssetID:       m.ImageAssetID,
			X:                  cells[i].X,
			Y:                  cells[i].Y,
			Width:              m.Width,
			Height:             m.Height,
			OwnerParticipantID: m.OwnerParticipantID,
			PartyKey:           m.ID,
			Icon:               m.Icon,
			Color:              m.Color,
		})
		if err != nil {
			// As in handleTokenCreate: what was made before the failure is
			// real, so the room still hears about it.
			slog.Error("ws: create party token failed", "error", err)
			h.sendError(ctx, c, "failed to add the party")
			break
		}
		created = append(created, token)
	}

	ids := make([]string, 0, len(created))
	for _, token := range created {
		h.broadcast(ctx, c.roomID, "token.created", tokenPayload(token))
		ids = append(ids, token.ID)
	}
	if len(ids) > 0 {
		h.send(ctx, c, "party.added", map[string]any{"tokenIds": ids})
	}
}
