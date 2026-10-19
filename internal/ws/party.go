package ws

import (
	"context"
	"encoding/json"
	"log/slog"

	"longtable/internal/store"
)

// The party: player characters a GM would otherwise re-create by hand on
// every new scene, since every token belongs to exactly one scene.
//
// **The party is a roster of characters, not a set of tokens.** Ticking a
// token into it gives it a party key (handleTokenUpdate mints one), and
// every copy made from it carries the same key. That is what the two
// obvious alternatives get wrong: flag the copies as well and Add party on
// the third scene copies everyone twice; flag only the originals and the
// party lives on one scene, gone the night that scene is deleted. Keyed,
// Add party puts one of each character down and leaves anyone already
// standing on the scene alone.

type partyAddRequest struct {
	SceneID string  `json:"sceneId"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
}

// handlePartyAdd puts a copy of every party character not already on a
// scene onto it, spread out from (x, y) the way a batch from the New
// token dialog is — though by footprint each, since a party mixes sizes.
//
// A copy is the character, not the evening: name, art, size and owner,
// plus visibility so a companion the GM is keeping hidden doesn't walk
// into the next room in plain view. Trackers and conditions start blank
// and position is wherever it lands. It copies the newest version of each
// character (ListPartyMembers says why), and it is a copy — editing one
// afterwards changes nothing on any other scene.
//
// The sender alone is then told which tokens were made, in party.added,
// so its undo stack can hold one entry per token the way a batch's does.
// It couldn't mint the ids itself as token.create's caller does: it has
// no way to know how many characters the server is about to put down.
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
		h.sendError(ctx, c, "No tokens are in the party yet.")
		return
	}
	existing, err := h.store.ListTokensForScene(req.SceneID)
	if err != nil {
		slog.Error("ws: list tokens for party failed", "error", err)
		h.sendError(ctx, c, "failed to add the party")
		return
	}

	// Anyone already here stays where they are, and doesn't get a twin —
	// which is also what makes a second press harmless, and lets a late
	// arrival's character be added by pressing it again.
	here := map[string]bool{}
	for _, t := range existing {
		if t.PartyKey != "" {
			here[t.PartyKey] = true
		}
	}
	var missing []store.Token
	for _, m := range members {
		if !here[m.PartyKey] {
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
			Visibility:         m.Visibility,
			PartyKey:           m.PartyKey,
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
		payload := tokenPayload(token)
		h.broadcastPerClient(ctx, c.roomID, "token.created", func(recipient *client) any {
			if token.Visibility == store.VisibilityHidden && recipient.participant.Role != store.RoleGM {
				return nil
			}
			return payload
		})
		ids = append(ids, token.ID)
	}
	if len(ids) > 0 {
		h.send(ctx, c, "party.added", map[string]any{"tokenIds": ids})
	}
}
