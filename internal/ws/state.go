package ws

import (
	"context"
	"log/slog"

	"longtable/internal/store"
)

// sendStateSync hydrates a freshly connected client with the room's
// current state: the active scene (if any) with its tokens and
// revealed fog, plus recent roll history, so it doesn't need to replay
// every event that has ever happened in the room.
func (h *Hub) sendStateSync(ctx context.Context, c *client, room store.Room) {
	payload := map[string]any{
		"room": map[string]any{
			"slug": room.Slug,
			"name": room.Name,
		},
		"you": map[string]any{
			"participantId": c.participant.ID,
			"displayName":   c.participant.DisplayName,
			"role":          string(c.participant.Role),
		},
	}

	rolls, err := h.store.ListRecentRolls(room.ID, 50)
	if err != nil {
		slog.Error("ws: list recent rolls failed", "error", err)
	} else {
		payload["rolls"] = rollPayloads(rolls)
	}

	if room.ActiveSceneID != nil {
		scene, err := h.store.GetScene(*room.ActiveSceneID)
		if err != nil {
			slog.Error("ws: load active scene failed", "error", err)
		} else {
			payload["scene"] = scenePayload(scene)

			if tokens, err := h.store.ListTokensForScene(scene.ID); err != nil {
				slog.Error("ws: list tokens failed", "error", err)
			} else {
				payload["tokens"] = tokenPayloads(tokens)
			}

			if fogCells, err := h.store.ListFogCells(scene.ID); err != nil {
				slog.Error("ws: list fog cells failed", "error", err)
			} else {
				payload["fogCells"] = fogCells
			}
		}
	}

	h.send(ctx, c, "state.sync", payload)
}

func tokenPayload(t store.Token) map[string]any {
	return map[string]any{
		"id":                 t.ID,
		"sceneId":            t.SceneID,
		"name":               t.Name,
		"imageAssetId":       t.ImageAssetID,
		"x":                  t.X,
		"y":                  t.Y,
		"width":              t.Width,
		"height":             t.Height,
		"ownerParticipantId": t.OwnerParticipantID,
		"visibility":         string(t.Visibility),
	}
}

func tokenPayloads(tokens []store.Token) []map[string]any {
	out := make([]map[string]any, len(tokens))
	for i, t := range tokens {
		out[i] = tokenPayload(t)
	}
	return out
}

func scenePayload(s store.Scene) map[string]any {
	return map[string]any{
		"id":          s.ID,
		"roomId":      s.RoomID,
		"name":        s.Name,
		"mapAssetId":  s.MapAssetID,
		"gridSize":    s.GridSize,
		"gridOffsetX": s.GridOffsetX,
		"gridOffsetY": s.GridOffsetY,
		"width":       s.Width,
		"height":      s.Height,
	}
}

func rollPayload(r store.Roll) map[string]any {
	return map[string]any{
		"id":              r.ID,
		"participantName": r.ParticipantName,
		"expression":      r.Expression,
		"result":          r.Result,
		"breakdown":       r.Breakdown,
		"createdAt":       r.CreatedAt,
	}
}

func rollPayloads(rolls []store.Roll) []map[string]any {
	out := make([]map[string]any, len(rolls))
	for i, r := range rolls {
		out[i] = rollPayload(r)
	}
	return out
}
