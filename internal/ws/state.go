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
// roomPayload is the room as every client is told about it — never the
// password hash, and exhaustive rather than a marshalled struct, for the
// same reason participantPayload is. Shared by state.sync and
// room.updated so a setting can't arrive on one and not the other.
func roomPayload(room store.Room) map[string]any {
	return map[string]any{
		"slug":              room.Slug,
		"name":              room.Name,
		"ownerOnlyMovement": room.OwnerOnlyMovement,
	}
}

func (h *Hub) sendStateSync(ctx context.Context, c *client, room store.Room) {
	payload := map[string]any{
		"room": roomPayload(room),
		"you": map[string]any{
			"participantId": c.participant.ID,
			"displayName":   c.participant.DisplayName,
			"role":          string(c.participant.Role),
		},
	}

	// The roster and the live subset are sent separately and come from
	// different places on purpose: the roster is a table, connectivity is
	// the hub's memory. Conflating them into one "is online" flag per row
	// would make the offline half unrepresentable, and the offline half is
	// exactly who a GM is assigning tokens to before a session.
	participants, err := h.store.ListParticipantsForRoom(room.ID)
	if err != nil {
		slog.Error("ws: list participants failed", "error", err)
	} else {
		payload["participants"] = participantPayloads(participants)
	}
	payload["connectedParticipantIds"] = h.ConnectedParticipantIDs(room.ID)

	messages, err := h.store.ListRecentMessages(room.ID, 50)
	if err != nil {
		slog.Error("ws: list recent messages failed", "error", err)
	} else {
		payload["messages"] = messagePayloads(messages, c.participant.ID)
	}

	// Every scene in the room, not just the active one: the scene picker
	// needs the whole list, and it changes rarely enough that carrying it
	// in the initial sync beats a second round trip on open.
	scenes, err := h.store.ListScenesForRoom(room.ID)
	if err != nil {
		slog.Error("ws: list scenes failed", "error", err)
	} else {
		payload["scenes"] = scenePayloads(scenes)
	}

	// The turn order belongs to the room rather than to a scene, so it
	// rides here beside the roster rather than inside the scene picture —
	// a GM flipping scenes mid-fight doesn't reload the encounter.
	payload["initiative"] = h.initiativeStatePayload(room.ID, c.participant.Role)

	if room.ActiveSceneID != nil {
		sceneState, err := h.sceneStatePayload(*room.ActiveSceneID, c.participant.Role)
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

// sceneStatePayload builds the {scene, tokens, fogChunks} triple used
// both to hydrate a freshly connected client (as part of state.sync)
// and to tell already-connected clients about a newly activated scene
// (scene.activated) — both cases need the same full picture, not just
// a bare scene ID. Tokens marked hidden are only included for role ==
// RoleGM; players never receive them, not even in filtered-out form.
func (h *Hub) sceneStatePayload(sceneID string, role store.Role) (map[string]any, error) {
	scene, err := h.store.GetScene(sceneID)
	if err != nil {
		return nil, err
	}

	tokens, err := h.store.ListTokensForScene(sceneID)
	if err != nil {
		return nil, err
	}
	if role != store.RoleGM {
		tokens = visibleTokensOnly(tokens)
	}

	fogChunks, err := h.store.ListFogChunks(sceneID)
	if err != nil {
		return nil, err
	}

	drawings, err := h.store.ListDrawingsForScene(sceneID)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"scene":     scenePayload(scene),
		"tokens":    tokenPayloads(tokens),
		"fogChunks": fogChunks,
		"drawings":  drawingPayloads(drawings),
	}, nil
}

func visibleTokensOnly(tokens []store.Token) []store.Token {
	out := make([]store.Token, 0, len(tokens))
	for _, t := range tokens {
		if t.Visibility != store.VisibilityHidden {
			out = append(out, t)
		}
	}
	return out
}

// participantPayload is deliberately three fields. store.Participant
// also carries SessionToken, which is a credential and must never reach
// another client — ListParticipantsForRoom doesn't even load it, and
// this is the second half of that: an exhaustive struct-to-map here
// rather than marshalling the struct, so a field added to the model
// can't silently start being broadcast.
func participantPayload(p store.Participant) map[string]any {
	return map[string]any{
		"id":          p.ID,
		"displayName": p.DisplayName,
		"role":        string(p.Role),
	}
}

func participantPayloads(participants []store.Participant) []map[string]any {
	out := make([]map[string]any, len(participants))
	for i, p := range participants {
		out[i] = participantPayload(p)
	}
	return out
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
		// Sent to everyone who is sent the token at all. A hidden token is
		// already withheld whole, so there is nothing here that needs
		// filtering a second time — and a Player seeing a monster's hit
		// points is a table's choice, not the protocol's.
		"trackers":   trackerPayloads(t.Trackers),
		"conditions": t.Conditions,
	}
}

// trackerPayloads is an exhaustive struct-to-map like participantPayload
// rather than a marshalled struct, so the tracker model can grow a field
// (a maximum, one day) without it silently starting to broadcast.
func trackerPayloads(trackers []store.Tracker) []map[string]any {
	out := make([]map[string]any, len(trackers))
	for i, tr := range trackers {
		out[i] = map[string]any{"label": tr.Label, "value": tr.Value}
	}
	return out
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

func scenePayloads(scenes []store.Scene) []map[string]any {
	out := make([]map[string]any, len(scenes))
	for i, s := range scenes {
		out[i] = scenePayload(s)
	}
	return out
}

func drawingPayload(d store.Drawing) map[string]any {
	return map[string]any{
		"id":                     d.ID,
		"sceneId":                d.SceneID,
		"kind":                   string(d.Kind),
		"points":                 d.Points,
		"color":                  d.Color,
		"filled":                 d.Filled,
		"strokeWidth":            d.StrokeWidth,
		"createdByParticipantId": d.CreatedByParticipantID,
	}
}

func drawingPayloads(drawings []store.Drawing) []map[string]any {
	out := make([]map[string]any, len(drawings))
	for i, d := range drawings {
		out[i] = drawingPayload(d)
	}
	return out
}

// messagePayload builds a message's payload for a specific viewer. A
// deleted message is redacted (body and roll fields blanked) unless the
// viewer is the one who wrote it or the one who deleted it — the same
// two people who can still make sense of an "undo" of their own action
// — mirroring how a hidden token is withheld from everyone but the GM.
// viewerParticipantID is ignored while the message isn't deleted, since
// there's nothing to redact yet.
func messagePayload(m store.Message, viewerParticipantID string) map[string]any {
	body, rollExpression, rollResult, rollBreakdown := m.Body, m.RollExpression, m.RollResult, m.RollBreakdown
	if m.DeletedAt != nil && !messageVisibleTo(m, viewerParticipantID) {
		body, rollExpression, rollResult, rollBreakdown = "", nil, nil, nil
	}
	return map[string]any{
		"id":              m.ID,
		"participantId":   m.ParticipantID,
		"participantName": m.ParticipantName,
		"kind":            string(m.Kind),
		"body":            body,
		"rollExpression":  rollExpression,
		"rollResult":      rollResult,
		"rollBreakdown":   rollBreakdown,
		"createdAt":       m.CreatedAt,
		"deleted":         m.DeletedAt != nil,
	}
}

// messageVisibleTo reports whether a deleted message's original content
// may still be shown to viewerParticipantID: whoever wrote it, or
// whoever deleted it. Those aren't always the same person — a GM may
// delete a Player's message, and that GM should still see what they
// just removed.
func messageVisibleTo(m store.Message, viewerParticipantID string) bool {
	if viewerParticipantID == "" {
		return false
	}
	if m.ParticipantID != nil && *m.ParticipantID == viewerParticipantID {
		return true
	}
	return m.DeletedByParticipantID != nil && *m.DeletedByParticipantID == viewerParticipantID
}

func messagePayloads(messages []store.Message, viewerParticipantID string) []map[string]any {
	out := make([]map[string]any, len(messages))
	for i, m := range messages {
		out[i] = messagePayload(m, viewerParticipantID)
	}
	return out
}
