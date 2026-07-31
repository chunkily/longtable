package ws

import (
	"encoding/json"
	"testing"

	"longtable/internal/store"
)

// fogTestRoom is a room with a GM and a Player both connected and past
// their state.sync, plus a scene whose bounds are a tidy 3x2 grid of
// 70px cells — small enough to assert on every cell reveal-all produces.
type fogTestRoom struct {
	ts     *testServer
	room   store.Room
	scene  store.Scene
	gm     *testClient
	player *testClient
}

func newFogTestRoom(t *testing.T) *fogTestRoom {
	t.Helper()

	ts := newTestServer(t)
	room, gm, err := ts.store.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	player, err := ts.store.JoinRoom(room.ID, "Bob")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	scene, err := ts.store.CreateScene(room.ID, "Scene", nil, 70, 210, 140)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	if err := ts.store.SetActiveScene(room.ID, scene.ID); err != nil {
		t.Fatalf("SetActiveScene: %v", err)
	}

	f := &fogTestRoom{
		ts:     ts,
		room:   room,
		scene:  scene,
		gm:     ts.connect(t, room.Slug, gm.SessionToken),
		player: ts.connect(t, room.Slug, player.SessionToken),
	}
	f.gm.readEnvelope(t)     // state.sync
	f.player.readEnvelope(t) // state.sync
	return f
}

// reveal paints cells through the protocol rather than the store, so the
// tests below start from fog a GM actually revealed, and drains both
// clients' echoes.
func (f *fogTestRoom) reveal(t *testing.T, cells []store.FogCell) {
	t.Helper()

	f.gm.send(t, "fog.reveal", map[string]any{"sceneId": f.scene.ID, "cells": cells})
	f.gm.readEnvelope(t)
	f.player.readEnvelope(t)
}

func (f *fogTestRoom) fogCells(t *testing.T) []store.FogCell {
	t.Helper()

	cells, err := f.ts.store.ListFogCells(f.scene.ID)
	if err != nil {
		t.Fatalf("ListFogCells: %v", err)
	}
	return cells
}

func fogCellsFromPayload(t *testing.T, env envelope) []store.FogCell {
	t.Helper()

	var payload struct {
		SceneID string          `json:"sceneId"`
		Cells   []store.FogCell `json:"cells"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal %s payload: %v", env.Type, err)
	}
	return payload.Cells
}

func errorMessage(t *testing.T, env envelope) string {
	t.Helper()

	if env.Type != "error" {
		t.Fatalf("type = %q, want error", env.Type)
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal error payload: %v", err)
	}
	return payload.Message
}

func TestFogHide_RemovesTheCellsAndTellsTheWholeRoom(t *testing.T) {
	f := newFogTestRoom(t)
	f.reveal(t, []store.FogCell{{X: 0, Y: 0}, {X: 1, Y: 0}})

	f.gm.send(t, "fog.hide", map[string]any{
		"sceneId": f.scene.ID,
		"cells":   []store.FogCell{{X: 0, Y: 0}},
	})

	env := f.gm.readEnvelope(t)
	if env.Type != "fog.hidden" {
		t.Fatalf("type = %q, want fog.hidden", env.Type)
	}
	if cells := fogCellsFromPayload(t, env); len(cells) != 1 || cells[0] != (store.FogCell{X: 0, Y: 0}) {
		t.Fatalf("cells = %+v, want just (0,0)", cells)
	}

	// A Player has to be told too, or the cell stays visible on their map
	// even though the server considers it hidden.
	if env := f.player.readEnvelope(t); env.Type != "fog.hidden" {
		t.Fatalf("player got %q, want fog.hidden", env.Type)
	}

	if got := f.fogCells(t); len(got) != 1 || got[0] != (store.FogCell{X: 1, Y: 0}) {
		t.Fatalf("stored cells = %+v, want only (1,0) still revealed", got)
	}
}

func TestFogHide_PlayerMayNot(t *testing.T) {
	f := newFogTestRoom(t)
	f.reveal(t, []store.FogCell{{X: 0, Y: 0}})

	f.player.send(t, "fog.hide", map[string]any{
		"sceneId": f.scene.ID,
		"cells":   []store.FogCell{{X: 0, Y: 0}},
	})

	if msg := errorMessage(t, f.player.readEnvelope(t)); msg == "" {
		t.Fatal("expected a refusal message")
	}
	if got := f.fogCells(t); len(got) != 1 {
		t.Fatalf("stored cells = %+v, want the reveal untouched", got)
	}
}

// A scene id from another room must fail exactly like one that doesn't
// exist, so the error can't be used to probe what other rooms hold.
func TestFogHide_SceneFromAnotherRoomFailsLikeAMissingOne(t *testing.T) {
	f := newFogTestRoom(t)

	otherRoom, _, err := f.ts.store.CreateRoom("Other", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	otherScene, err := f.ts.store.CreateScene(otherRoom.ID, "Theirs", nil, 70, 210, 140)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	if err := f.ts.store.RevealCells(otherScene.ID, []store.FogCell{{X: 0, Y: 0}}); err != nil {
		t.Fatalf("RevealCells: %v", err)
	}

	f.gm.send(t, "fog.hide", map[string]any{
		"sceneId": otherScene.ID,
		"cells":   []store.FogCell{{X: 0, Y: 0}},
	})
	fromOtherRoom := errorMessage(t, f.gm.readEnvelope(t))

	f.gm.send(t, "fog.hide", map[string]any{
		"sceneId": "nosuch",
		"cells":   []store.FogCell{{X: 0, Y: 0}},
	})
	fromMissing := errorMessage(t, f.gm.readEnvelope(t))

	if fromOtherRoom != fromMissing {
		t.Fatalf("errors differ: %q vs %q — the two must be indistinguishable", fromOtherRoom, fromMissing)
	}

	cells, err := f.ts.store.ListFogCells(otherScene.ID)
	if err != nil {
		t.Fatalf("ListFogCells: %v", err)
	}
	if len(cells) != 1 {
		t.Fatalf("other room's cells = %+v, want untouched", cells)
	}
}

func TestFogHide_RejectsPayloadWithNoCells(t *testing.T) {
	f := newFogTestRoom(t)

	f.gm.send(t, "fog.hide", map[string]any{"sceneId": f.scene.ID})

	if msg := errorMessage(t, f.gm.readEnvelope(t)); msg != "invalid fog.hide payload" {
		t.Fatalf("message = %q, want the invalid-payload refusal", msg)
	}
}

func TestFogRevealAll_CoversEveryCellInBoundsAndReusesFogRevealed(t *testing.T) {
	f := newFogTestRoom(t)

	f.gm.send(t, "fog.revealAll", map[string]any{"sceneId": f.scene.ID})

	env := f.gm.readEnvelope(t)
	// Deliberately the same event a hand-painted reveal broadcasts, so
	// clients need no separate case for it.
	if env.Type != "fog.revealed" {
		t.Fatalf("type = %q, want fog.revealed", env.Type)
	}
	// 210x140 at gridSize 70 is 3 columns by 2 rows.
	if cells := fogCellsFromPayload(t, env); len(cells) != 6 {
		t.Fatalf("len(cells) = %d, want 6", len(cells))
	}
	if env := f.player.readEnvelope(t); env.Type != "fog.revealed" {
		t.Fatalf("player got %q, want fog.revealed", env.Type)
	}

	if got := f.fogCells(t); len(got) != 6 {
		t.Fatalf("stored cells = %d, want all 6 persisted", len(got))
	}
}

// Revealing everything twice, or over cells already painted by hand,
// must not duplicate rows — RevealCells is the same idempotent insert.
func TestFogRevealAll_IsIdempotentOverAlreadyRevealedCells(t *testing.T) {
	f := newFogTestRoom(t)
	f.reveal(t, []store.FogCell{{X: 1, Y: 1}})

	for range 2 {
		f.gm.send(t, "fog.revealAll", map[string]any{"sceneId": f.scene.ID})
		f.gm.readEnvelope(t)
		f.player.readEnvelope(t)
	}

	if got := f.fogCells(t); len(got) != 6 {
		t.Fatalf("stored cells = %d, want 6", len(got))
	}
}

func TestFogRevealAll_SceneWithoutBoundsIsRefused(t *testing.T) {
	f := newFogTestRoom(t)

	// A scene created without width/height has no map to reveal — there's
	// nothing to enumerate, so this has to say so rather than silently
	// revealing nothing.
	unbounded, err := f.ts.store.CreateScene(f.room.ID, "No map", nil, 70, 0, 0)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	f.gm.send(t, "fog.revealAll", map[string]any{"sceneId": unbounded.ID})

	if msg := errorMessage(t, f.gm.readEnvelope(t)); msg == "" {
		t.Fatal("expected a message explaining there are no bounds")
	}
}

func TestFogRevealAll_PlayerMayNot(t *testing.T) {
	f := newFogTestRoom(t)

	f.player.send(t, "fog.revealAll", map[string]any{"sceneId": f.scene.ID})

	if msg := errorMessage(t, f.player.readEnvelope(t)); msg == "" {
		t.Fatal("expected a refusal message")
	}
	if got := f.fogCells(t); len(got) != 0 {
		t.Fatalf("stored cells = %+v, want none revealed", got)
	}
}

func TestFogReset_ClearsTheSceneForEveryone(t *testing.T) {
	f := newFogTestRoom(t)
	f.reveal(t, []store.FogCell{{X: 0, Y: 0}, {X: 1, Y: 1}})

	f.gm.send(t, "fog.reset", map[string]any{"sceneId": f.scene.ID})

	if env := f.gm.readEnvelope(t); env.Type != "fog.reset" {
		t.Fatalf("type = %q, want fog.reset", env.Type)
	}
	if env := f.player.readEnvelope(t); env.Type != "fog.reset" {
		t.Fatalf("player got %q, want fog.reset", env.Type)
	}

	if got := f.fogCells(t); len(got) != 0 {
		t.Fatalf("stored cells = %+v, want none", got)
	}
}

func TestFogReset_PlayerMayNot(t *testing.T) {
	f := newFogTestRoom(t)
	f.reveal(t, []store.FogCell{{X: 0, Y: 0}})

	f.player.send(t, "fog.reset", map[string]any{"sceneId": f.scene.ID})

	if msg := errorMessage(t, f.player.readEnvelope(t)); msg == "" {
		t.Fatal("expected a refusal message")
	}
	if got := f.fogCells(t); len(got) != 1 {
		t.Fatalf("stored cells = %+v, want the reveal untouched", got)
	}
}

func TestSceneFogCells(t *testing.T) {
	tests := []struct {
		name           string
		gridSize, w, h int
		wantCells      int
		wantLast       store.FogCell
		wantErr        bool
	}{
		{name: "exact fit", gridSize: 70, w: 210, h: 140, wantCells: 6, wantLast: store.FogCell{X: 2, Y: 1}},
		// A map whose last row and column of squares are clipped still gets
		// them, or reveal-all would leave a covered strip along two edges.
		{name: "partial cells round up", gridSize: 70, w: 211, h: 141, wantCells: 12, wantLast: store.FogCell{X: 3, Y: 2}},
		{name: "single cell", gridSize: 70, w: 1, h: 1, wantCells: 1, wantLast: store.FogCell{X: 0, Y: 0}},
		{name: "no bounds", gridSize: 70, w: 0, h: 0, wantErr: true},
		{name: "no grid", gridSize: 0, w: 210, h: 140, wantErr: true},
		{name: "past the cap", gridSize: 1, w: 1000, h: 1000, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cells, err := sceneFogCells(store.Scene{GridSize: tt.gridSize, Width: tt.w, Height: tt.h})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("err = nil, want a refusal (got %d cells)", len(cells))
				}
				return
			}
			if err != nil {
				t.Fatalf("sceneFogCells: %v", err)
			}
			if len(cells) != tt.wantCells {
				t.Fatalf("len(cells) = %d, want %d", len(cells), tt.wantCells)
			}
			if cells[0] != (store.FogCell{X: 0, Y: 0}) {
				t.Fatalf("first cell = %+v, want the origin", cells[0])
			}
			if cells[len(cells)-1] != tt.wantLast {
				t.Fatalf("last cell = %+v, want %+v", cells[len(cells)-1], tt.wantLast)
			}
		})
	}
}
