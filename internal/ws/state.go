package ws

import (
	"context"
	"log/slog"

	"longtable/internal/store"
)

// sendStateSync hydrates a freshly connected client with the room's
// current state: the active scene (if any) with its tokens and
// revealed fog, plus recent chat history, so it doesn't need to replay
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

	messages, err := h.store.ListRecentMessages(room.ID, 50)
	if err != nil {
		slog.Error("ws: list recent messages failed", "error", err)
	} else {
		payload["messages"] = messagePayloads(messages)
	}

	if room.ActiveSceneID != nil {
		sceneState, err := h.sceneStatePayload(*room.ActiveSceneID)
		if err != nil {
			slog.Error("ws: load active scene state failed", "error", err)
		} else {
			for k, v := range sceneState {
				payload[k] = v
			}
		}
	}

	h.send(ctx, c, "state.sync", payload)
}

// sceneStatePayload builds the {scene, tokens, fogCells} triple used
// both to hydrate a freshly connected client (as part of state.sync)
// and to tell already-connected clients about a newly activated scene
// (scene.activated) — both cases need the same full picture, not just
// a bare scene ID.
func (h *Hub) sceneStatePayload(sceneID string) (map[string]any, error) {
	scene, err := h.store.GetScene(sceneID)
	if err != nil {
		return nil, err
	}

	tokens, err := h.store.ListTokensForScene(sceneID)
	if err != nil {
		return nil, err
	}

	fogCells, err := h.store.ListFogCells(sceneID)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"scene":    scenePayload(scene),
		"tokens":   tokenPayloads(tokens),
		"fogCells": fogCells,
	}, nil
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

func messagePayload(m store.Message) map[string]any {
	return map[string]any{
		"id":              m.ID,
		"participantId":   m.ParticipantID,
		"participantName": m.ParticipantName,
		"kind":            string(m.Kind),
		"body":            m.Body,
		"rollExpression":  m.RollExpression,
		"rollResult":      m.RollResult,
		"rollBreakdown":   m.RollBreakdown,
		"createdAt":       m.CreatedAt,
	}
}

func messagePayloads(messages []store.Message) []map[string]any {
	out := make([]map[string]any, len(messages))
	for i, m := range messages {
		out[i] = messagePayload(m)
	}
	return out
}
