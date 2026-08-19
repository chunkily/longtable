package ws

import (
	"encoding/json"
	"testing"
	"time"

	"longtable/internal/store"
)

// fogTestRoom is a room with a GM and a Player both connected and past
// their state.sync, plus a scene whose bounds are a tidy 3x2 grid of
// 70px cells — small enough to assert on every chunk a whole-scene
// operation produces.
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
	player, err := ts.store.JoinRoom(room.ID, "Bob", "")
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

// hide covers cells through the protocol rather than the store, so the
// tests below start from fog a GM actually painted, and drains both
// clients' echoes.
func (f *fogTestRoom) hide(t *testing.T, cells []store.FogCell) {
	t.Helper()

	f.gm.send(t, "fog.hide", map[string]any{"sceneId": f.scene.ID, "cells": cells})
	f.gm.readEnvelope(t)
	f.player.readEnvelope(t)
}

func (f *fogTestRoom) fogChunks(t *testing.T) []store.FogChunk {
	t.Helper()

	chunks, err := f.ts.store.ListFogChunks(f.scene.ID)
	if err != nil {
		t.Fatalf("ListFogChunks: %v", err)
	}
	return chunks
}

func fogChunksFromPayload(t *testing.T, env envelope) []store.FogChunk {
	t.Helper()

	var payload struct {
		SceneID string           `json:"sceneId"`
		Chunks  []store.FogChunk `json:"chunks"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal %s payload: %v", env.Type, err)
	}
	return payload.Chunks
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

func TestFogHide_PacksTheCellsAndTellsTheWholeRoom(t *testing.T) {
	f := newFogTestRoom(t)

	f.gm.send(t, "fog.hide", map[string]any{
		"sceneId": f.scene.ID,
		"cells":   []store.FogCell{{X: 0, Y: 0}, {X: 1, Y: 0}},
	})

	env := f.gm.readEnvelope(t)
	if env.Type != "fog.hidden" {
		t.Fatalf("type = %q, want fog.hidden", env.Type)
	}
	// Two adjacent cells are two bits of one chunk, not two payload
	// entries — the packing goes over the wire, not just into the table.
	chunks := fogChunksFromPayload(t, env)
	if len(chunks) != 1 || chunks[0] != (store.FogChunk{Y: 0, ChunkX: 0, Mask: 0b11}) {
		t.Fatalf("chunks = %+v, want one chunk with bits 0 and 1 set", chunks)
	}

	// A Player has to be told too, or the cells stay visible on their map
	// even though the server considers them covered.
	if env := f.player.readEnvelope(t); env.Type != "fog.hidden" {
		t.Fatalf("player got %q, want fog.hidden", env.Type)
	}

	if got := f.fogChunks(t); len(got) != 1 || got[0].Mask != 0b11 {
		t.Fatalf("stored chunks = %+v, want the two cells hidden", got)
	}
}

func TestFogReveal_ClearsTheBitsAndTellsTheWholeRoom(t *testing.T) {
	f := newFogTestRoom(t)
	f.hide(t, []store.FogCell{{X: 0, Y: 0}, {X: 1, Y: 0}})

	f.gm.send(t, "fog.reveal", map[string]any{
		"sceneId": f.scene.ID,
		"cells":   []store.FogCell{{X: 0, Y: 0}},
	})

	env := f.gm.readEnvelope(t)
	if env.Type != "fog.revealed" {
		t.Fatalf("type = %q, want fog.revealed", env.Type)
	}
	// The chunk's new mask, not the cells that changed: the client takes
	// the value rather than working out a delta of its own.
	chunks := fogChunksFromPayload(t, env)
	if len(chunks) != 1 || chunks[0].Mask != 0b10 {
		t.Fatalf("chunks = %+v, want the chunk carrying only x=1 now", chunks)
	}

	if env := f.player.readEnvelope(t); env.Type != "fog.revealed" {
		t.Fatalf("player got %q, want fog.revealed", env.Type)
	}
}

// Both painting commands are idempotent, which is what lets the
// rectangle tool send every cell in its box. A drag that changes nothing
// must also *say* nothing rather than waking every client up for it.
func TestFogPaint_SaysNothingWhenNoChunkChanged(t *testing.T) {
	f := newFogTestRoom(t)
	f.hide(t, []store.FogCell{{X: 0, Y: 0}})

	f.gm.send(t, "fog.hide", map[string]any{
		"sceneId": f.scene.ID,
		"cells":   []store.FogCell{{X: 0, Y: 0}},
	})

	f.gm.expectNoMessage(t, 300*time.Millisecond)
}

func TestFogHide_PlayerMayNot(t *testing.T) {
	f := newFogTestRoom(t)

	f.player.send(t, "fog.hide", map[string]any{
		"sceneId": f.scene.ID,
		"cells":   []store.FogCell{{X: 0, Y: 0}},
	})

	if msg := errorMessage(t, f.player.readEnvelope(t)); msg == "" {
		t.Fatal("expected a refusal message")
	}
	if got := f.fogChunks(t); len(got) != 0 {
		t.Fatalf("stored chunks = %+v, want nothing hidden", got)
	}
}

func TestFogReveal_PlayerMayNot(t *testing.T) {
	f := newFogTestRoom(t)
	f.hide(t, []store.FogCell{{X: 0, Y: 0}})

	f.player.send(t, "fog.reveal", map[string]any{
		"sceneId": f.scene.ID,
		"cells":   []store.FogCell{{X: 0, Y: 0}},
	})

	if msg := errorMessage(t, f.player.readEnvelope(t)); msg == "" {
		t.Fatal("expected a refusal message")
	}
	if got := f.fogChunks(t); len(got) != 1 {
		t.Fatalf("stored chunks = %+v, want the fog untouched", got)
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
	if _, err := f.ts.store.HideCells(otherScene.ID, []store.FogCell{{X: 0, Y: 0}}); err != nil {
		t.Fatalf("HideCells: %v", err)
	}

	f.gm.send(t, "fog.hide", map[string]any{
		"sceneId": otherScene.ID,
		"cells":   []store.FogCell{{X: 1, Y: 1}},
	})
	fromOtherRoom := errorMessage(t, f.gm.readEnvelope(t))

	f.gm.send(t, "fog.hide", map[string]any{
		"sceneId": "nosuch",
		"cells":   []store.FogCell{{X: 1, Y: 1}},
	})
	fromMissing := errorMessage(t, f.gm.readEnvelope(t))

	if fromOtherRoom != fromMissing {
		t.Fatalf("errors differ: %q vs %q — the two must be indistinguishable", fromOtherRoom, fromMissing)
	}

	chunks, err := f.ts.store.ListFogChunks(otherScene.ID)
	if err != nil {
		t.Fatalf("ListFogChunks: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Mask != 0b1 {
		t.Fatalf("other room's chunks = %+v, want untouched", chunks)
	}
}

func TestFogHide_RejectsPayloadWithNoCells(t *testing.T) {
	f := newFogTestRoom(t)

	f.gm.send(t, "fog.hide", map[string]any{"sceneId": f.scene.ID})

	if msg := errorMessage(t, f.gm.readEnvelope(t)); msg != "invalid fog.hide payload" {
		t.Fatalf("message = %q, want the invalid-payload refusal", msg)
	}
}

func TestFogRevealAll_EmptiesTheSceneAndReusesFogRevealed(t *testing.T) {
	f := newFogTestRoom(t)
	f.hide(t, []store.FogCell{{X: 0, Y: 0}, {X: 2, Y: 1}})

	f.gm.send(t, "fog.revealAll", map[string]any{"sceneId": f.scene.ID})

	env := f.gm.readEnvelope(t)
	// Deliberately the same event a painted reveal broadcasts, so clients
	// need no separate case for it.
	if env.Type != "fog.revealed" {
		t.Fatalf("type = %q, want fog.revealed", env.Type)
	}
	// Both chunks come back zeroed rather than simply omitted: the client
	// merges by key, so a chunk it already holds has to be told it is
	// empty now.
	chunks := fogChunksFromPayload(t, env)
	if len(chunks) != 2 {
		t.Fatalf("chunks = %+v, want both rows reported", chunks)
	}
	for _, c := range chunks {
		if c.Mask != 0 {
			t.Fatalf("chunk = %+v, want mask 0", c)
		}
	}
	if env := f.player.readEnvelope(t); env.Type != "fog.revealed" {
		t.Fatalf("player got %q, want fog.revealed", env.Type)
	}

	if got := f.fogChunks(t); len(got) != 0 {
		t.Fatalf("stored chunks = %+v, want none", got)
	}
}

// Reveal-all needs no scene bounds at all now — it only has to describe
// the chunks that actually hold fog. A scene with no width or height
// used to be refused by this command, and is the case that proves the
// cap moved to fog.reset.
func TestFogRevealAll_WorksOnASceneWithNoBounds(t *testing.T) {
	f := newFogTestRoom(t)

	unbounded, err := f.ts.store.CreateScene(f.room.ID, "No map", nil, 70, 0, 0)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	if _, err := f.ts.store.HideCells(unbounded.ID, []store.FogCell{{X: 5, Y: 5}}); err != nil {
		t.Fatalf("HideCells: %v", err)
	}

	f.gm.send(t, "fog.revealAll", map[string]any{"sceneId": unbounded.ID})

	if env := f.gm.readEnvelope(t); env.Type != "fog.revealed" {
		t.Fatalf("type = %q, want fog.revealed rather than a refusal", env.Type)
	}
	f.player.readEnvelope(t)

	chunks, err := f.ts.store.ListFogChunks(unbounded.ID)
	if err != nil {
		t.Fatalf("ListFogChunks: %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("chunks = %+v, want the fog cleared", chunks)
	}
}

// Nothing to clear is nothing to say, rather than an empty broadcast.
func TestFogRevealAll_SaysNothingOnASceneWithNoFog(t *testing.T) {
	f := newFogTestRoom(t)

	f.gm.send(t, "fog.revealAll", map[string]any{"sceneId": f.scene.ID})

	f.gm.expectNoMessage(t, 300*time.Millisecond)
}

func TestFogRevealAll_PlayerMayNot(t *testing.T) {
	f := newFogTestRoom(t)
	f.hide(t, []store.FogCell{{X: 0, Y: 0}})

	f.player.send(t, "fog.revealAll", map[string]any{"sceneId": f.scene.ID})

	if msg := errorMessage(t, f.player.readEnvelope(t)); msg == "" {
		t.Fatal("expected a refusal message")
	}
	if got := f.fogChunks(t); len(got) != 1 {
		t.Fatalf("stored chunks = %+v, want the fog untouched", got)
	}
}

func TestFogReset_CoversEveryCellInBoundsAndReusesFogHidden(t *testing.T) {
	f := newFogTestRoom(t)

	f.gm.send(t, "fog.reset", map[string]any{"sceneId": f.scene.ID})

	env := f.gm.readEnvelope(t)
	// Covering everything is covering every chunk, which is what a painted
	// hide already says — so there is no fog.reset event any more.
	if env.Type != "fog.hidden" {
		t.Fatalf("type = %q, want fog.hidden", env.Type)
	}
	// 210x140 at gridSize 70 is 3 columns by 2 rows: one chunk per row,
	// each carrying only the 3 bits that are really there.
	chunks := fogChunksFromPayload(t, env)
	if len(chunks) != 2 {
		t.Fatalf("chunks = %+v, want one per row", chunks)
	}
	for _, c := range chunks {
		if c.Mask != 0b111 {
			t.Fatalf("chunk = %+v, want exactly 3 columns covered", c)
		}
	}
	if env := f.player.readEnvelope(t); env.Type != "fog.hidden" {
		t.Fatalf("player got %q, want fog.hidden", env.Type)
	}

	if got := f.fogChunks(t); len(got) != 2 {
		t.Fatalf("stored chunks = %+v, want both rows persisted", got)
	}
}

func TestFogReset_SceneWithoutBoundsIsRefused(t *testing.T) {
	f := newFogTestRoom(t)

	// A scene created without width/height has no map to cover — there's
	// nothing to enumerate, so this has to say so rather than silently
	// covering nothing.
	unbounded, err := f.ts.store.CreateScene(f.room.ID, "No map", nil, 70, 0, 0)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}

	f.gm.send(t, "fog.reset", map[string]any{"sceneId": unbounded.ID})

	if msg := errorMessage(t, f.gm.readEnvelope(t)); msg == "" {
		t.Fatal("expected a message explaining there are no bounds")
	}
}

func TestFogReset_PlayerMayNot(t *testing.T) {
	f := newFogTestRoom(t)

	f.player.send(t, "fog.reset", map[string]any{"sceneId": f.scene.ID})

	if msg := errorMessage(t, f.player.readEnvelope(t)); msg == "" {
		t.Fatal("expected a refusal message")
	}
	if got := f.fogChunks(t); len(got) != 0 {
		t.Fatalf("stored chunks = %+v, want nothing covered", got)
	}
}

func TestSceneFogChunks(t *testing.T) {
	tests := []struct {
		name           string
		gridSize, w, h int
		wantChunks     int
		wantLastMask   uint32
		wantErr        bool
	}{
		// 3 columns fit in one chunk, with only 3 of its 32 bits real.
		{name: "narrower than a chunk", gridSize: 70, w: 210, h: 140, wantChunks: 2, wantLastMask: 0b111},
		// Exactly 32 columns fills a chunk edge to edge.
		{name: "exactly one chunk wide", gridSize: 10, w: 320, h: 10, wantChunks: 1, wantLastMask: ^uint32(0)},
		// 33 columns spills into a second chunk holding a single bit.
		{name: "one past a chunk", gridSize: 10, w: 330, h: 10, wantChunks: 2, wantLastMask: 0b1},
		// A map whose last row and column of squares are clipped still gets
		// them, or reset would leave a revealed strip along two edges.
		{name: "partial cells round up", gridSize: 70, w: 211, h: 141, wantChunks: 3, wantLastMask: 0b1111},
		{name: "no bounds", gridSize: 70, w: 0, h: 0, wantErr: true},
		{name: "no grid", gridSize: 0, w: 210, h: 140, wantErr: true},
		{name: "past the cap", gridSize: 1, w: 1000, h: 3000, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := sceneFogChunks(store.Scene{GridSize: tt.gridSize, Width: tt.w, Height: tt.h})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("err = nil, want a refusal (got %d chunks)", len(chunks))
				}
				return
			}
			if err != nil {
				t.Fatalf("sceneFogChunks: %v", err)
			}
			if len(chunks) != tt.wantChunks {
				t.Fatalf("len(chunks) = %d, want %d", len(chunks), tt.wantChunks)
			}
			if chunks[0].Y != 0 || chunks[0].ChunkX != 0 {
				t.Fatalf("first chunk = %+v, want the origin", chunks[0])
			}
			if got := chunks[len(chunks)-1].Mask; got != tt.wantLastMask {
				t.Fatalf("last mask = %b, want %b", got, tt.wantLastMask)
			}
		})
	}
}
