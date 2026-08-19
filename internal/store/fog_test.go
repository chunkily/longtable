package store

import "testing"

// twoScenesWithFog gives two scenes in one room with the same cells
// hidden — the setup that catches a reveal or a whole-scene operation
// that forgets to scope itself, since fog coordinates repeat across
// every scene.
func twoScenesWithFog(t *testing.T) (*Store, Scene, Scene) {
	t.Helper()

	s := newTestStore(t)
	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	cells := []FogCell{{X: 1, Y: 1}, {X: 2, Y: 2}}
	scenes := make([]Scene, 2)
	for i, name := range []string{"First", "Second"} {
		sc, err := s.CreateScene(room.ID, name, nil, 70, 700, 700)
		if err != nil {
			t.Fatalf("CreateScene(%s): %v", name, err)
		}
		if _, err := s.HideCells(sc.ID, cells); err != nil {
			t.Fatalf("HideCells(%s): %v", name, err)
		}
		scenes[i] = sc
	}
	return s, scenes[0], scenes[1]
}

func sceneWithNoFog(t *testing.T) (*Store, Scene) {
	t.Helper()

	s := newTestStore(t)
	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	scene, err := s.CreateScene(room.ID, "Scene", nil, 70, 700, 700)
	if err != nil {
		t.Fatalf("CreateScene: %v", err)
	}
	return s, scene
}

// A scene nobody has painted on holds no rows at all, which is the whole
// reason fog stores what's hidden: "revealed everywhere" is the state a
// scene is created in, and it costs nothing to be in.
func TestFog_NewSceneHoldsNothing(t *testing.T) {
	s, scene := sceneWithNoFog(t)

	chunks, err := s.ListFogChunks(scene.ID)
	if err != nil {
		t.Fatalf("ListFogChunks: %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("chunks = %+v, want none — a new scene has no fog on it", chunks)
	}
}

// 32 cells in a row share one row in the table. That is the point of
// packing, so it's worth pinning directly rather than inferring from
// behaviour elsewhere.
func TestHideCells_PacksAWholeChunkIntoOneRow(t *testing.T) {
	s, scene := sceneWithNoFog(t)

	cells := make([]FogCell, 0, FogChunkWidth)
	for x := range FogChunkWidth {
		cells = append(cells, FogCell{X: x, Y: 3})
	}
	if _, err := s.HideCells(scene.ID, cells); err != nil {
		t.Fatalf("HideCells: %v", err)
	}

	chunks, err := s.ListFogChunks(scene.ID)
	if err != nil {
		t.Fatalf("ListFogChunks: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("chunks = %+v, want 32 cells packed into 1 row", chunks)
	}
	if chunks[0] != (FogChunk{Y: 3, ChunkX: 0, Mask: ^uint32(0)}) {
		t.Fatalf("chunk = %+v, want every bit of row 3's first chunk set", chunks[0])
	}

	// The 33rd cell starts a second chunk rather than overflowing the first.
	if _, err := s.HideCells(scene.ID, []FogCell{{X: FogChunkWidth, Y: 3}}); err != nil {
		t.Fatalf("HideCells(33rd): %v", err)
	}
	chunks, err = s.ListFogChunks(scene.ID)
	if err != nil {
		t.Fatalf("ListFogChunks: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("chunks = %+v, want a second chunk for x=32", chunks)
	}
}

// The grid is infinite, so fog can be painted left of or above the
// origin. Go's / and % truncate toward zero, which would fold x=-1 into
// chunk 0 alongside x=1 — chunkFor shifts and masks instead.
func TestHideCells_NegativeCoordinatesGetTheirOwnChunk(t *testing.T) {
	s, scene := sceneWithNoFog(t)

	if _, err := s.HideCells(scene.ID, []FogCell{{X: -1, Y: 0}, {X: 1, Y: 0}}); err != nil {
		t.Fatalf("HideCells: %v", err)
	}

	chunks, err := s.ListFogChunks(scene.ID)
	if err != nil {
		t.Fatalf("ListFogChunks: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("chunks = %+v, want x=-1 and x=1 in different chunks", chunks)
	}

	byChunkX := map[int]uint32{}
	for _, c := range chunks {
		byChunkX[c.ChunkX] = c.Mask
	}
	if got := byChunkX[-1]; got != 1<<31 {
		t.Fatalf("chunk -1 mask = %d, want the top bit set for x=-1", got)
	}
	if got := byChunkX[0]; got != 0b10 {
		t.Fatalf("chunk 0 mask = %d, want bit 1 set for x=1", got)
	}
}

func TestHideCells_ReportsOnlyChunksThatChanged(t *testing.T) {
	s, scene, _ := twoScenesWithFog(t)

	// Already hidden by the fixture, so this changes no bit. The rectangle
	// tool sends every cell in its box including ones already in the
	// target state, and broadcasting those is work every client would do
	// nothing with.
	changed, err := s.HideCells(scene.ID, []FogCell{{X: 1, Y: 1}})
	if err != nil {
		t.Fatalf("HideCells: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("changed = %+v, want nothing reported for an already-hidden cell", changed)
	}

	changed, err = s.HideCells(scene.ID, []FogCell{{X: 1, Y: 1}, {X: 4, Y: 1}})
	if err != nil {
		t.Fatalf("HideCells: %v", err)
	}
	if len(changed) != 1 || changed[0].Y != 1 || changed[0].Mask != 0b10010 {
		t.Fatalf("changed = %+v, want row 1's chunk carrying both x=1 and x=4", changed)
	}
}

func TestRevealCells_ClearsOnlyTheNamedCellsInThatScene(t *testing.T) {
	s, scene, other := twoScenesWithFog(t)

	if _, err := s.RevealCells(scene.ID, []FogCell{{X: 1, Y: 1}}); err != nil {
		t.Fatalf("RevealCells: %v", err)
	}

	chunks, err := s.ListFogChunks(scene.ID)
	if err != nil {
		t.Fatalf("ListFogChunks: %v", err)
	}
	// Row 1's chunk emptied and was dropped; row 2's still holds x=2.
	if len(chunks) != 1 || chunks[0].Y != 2 || chunks[0].Mask != 0b100 {
		t.Fatalf("chunks = %+v, want only row 2's (2,2) left", chunks)
	}

	// The other scene has a (1,1) of its own, which this must not reach.
	otherChunks, err := s.ListFogChunks(other.ID)
	if err != nil {
		t.Fatalf("ListFogChunks(other): %v", err)
	}
	if len(otherChunks) != 2 {
		t.Fatalf("other scene chunks = %+v, want both still hidden", otherChunks)
	}
}

// A chunk with nothing hidden left is deleted rather than kept at zero,
// so "no row" stays the only spelling of "all revealed". Two spellings
// would mean every read had to handle both.
func TestRevealCells_DropsAChunkItEmpties(t *testing.T) {
	s, scene, _ := twoScenesWithFog(t)

	changed, err := s.RevealCells(scene.ID, []FogCell{{X: 1, Y: 1}})
	if err != nil {
		t.Fatalf("RevealCells: %v", err)
	}
	if len(changed) != 1 || changed[0].Mask != 0 {
		t.Fatalf("changed = %+v, want the emptied chunk reported at mask 0", changed)
	}

	chunks, err := s.ListFogChunks(scene.ID)
	if err != nil {
		t.Fatalf("ListFogChunks: %v", err)
	}
	for _, c := range chunks {
		if c.Y == 1 {
			t.Fatalf("row 1 chunk = %+v, want it gone rather than stored at zero", c)
		}
	}
}

// Revealing what was never hidden is how a rectangle drag behaves over
// ground that's already clear, so it has to be silent rather than an
// error — and must report nothing, since nothing changed.
func TestRevealCells_AlreadyRevealedCellIsANoOp(t *testing.T) {
	s, scene, _ := twoScenesWithFog(t)

	changed, err := s.RevealCells(scene.ID, []FogCell{{X: 9, Y: 9}})
	if err != nil {
		t.Fatalf("RevealCells: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("changed = %+v, want nothing reported", changed)
	}

	chunks, err := s.ListFogChunks(scene.ID)
	if err != nil {
		t.Fatalf("ListFogChunks: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("chunks = %+v, want both untouched", chunks)
	}
}

func TestRevealAllCells_EmptiesOneSceneOnly(t *testing.T) {
	s, scene, other := twoScenesWithFog(t)

	cleared, err := s.RevealAllCells(scene.ID)
	if err != nil {
		t.Fatalf("RevealAllCells: %v", err)
	}
	// Reported zeroed rather than simply forgotten, so one merge rule
	// covers every fog event a client receives.
	if len(cleared) != 2 {
		t.Fatalf("cleared = %+v, want both chunks reported", cleared)
	}
	for _, c := range cleared {
		if c.Mask != 0 {
			t.Fatalf("cleared chunk = %+v, want mask 0", c)
		}
	}

	chunks, err := s.ListFogChunks(scene.ID)
	if err != nil {
		t.Fatalf("ListFogChunks: %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("chunks = %+v, want none", chunks)
	}

	otherChunks, err := s.ListFogChunks(other.ID)
	if err != nil {
		t.Fatalf("ListFogChunks(other): %v", err)
	}
	if len(otherChunks) != 2 {
		t.Fatalf("other scene chunks = %+v, want both still hidden", otherChunks)
	}
}

// Covering a whole scene replaces whatever was there. Fog outside the
// bounds being covered has to come back zeroed in the delta, or a client
// would keep drawing fog the server just deleted.
func TestHideAllCells_ReplacesAndReportsStraysOutsideTheNewSet(t *testing.T) {
	s, scene, other := twoScenesWithFog(t)

	// Fog left of the origin, outside any bounds a caller would enumerate.
	if _, err := s.HideCells(scene.ID, []FogCell{{X: -1, Y: 0}}); err != nil {
		t.Fatalf("HideCells(stray): %v", err)
	}

	full := []FogChunk{{Y: 0, ChunkX: 0, Mask: 0b111}}
	delta, err := s.HideAllCells(scene.ID, full)
	if err != nil {
		t.Fatalf("HideAllCells: %v", err)
	}

	chunks, err := s.ListFogChunks(scene.ID)
	if err != nil {
		t.Fatalf("ListFogChunks: %v", err)
	}
	if len(chunks) != 1 || chunks[0] != full[0] {
		t.Fatalf("chunks = %+v, want exactly the new set", chunks)
	}

	// The delta carries the new chunk plus every chunk it displaced,
	// zeroed: rows 1 and 2 from the fixture, and the stray at chunk -1.
	zeroed := 0
	for _, c := range delta {
		if c.Mask == 0 {
			zeroed++
		}
	}
	if len(delta) != 4 || zeroed != 3 {
		t.Fatalf("delta = %+v, want the new chunk plus 3 zeroed ones", delta)
	}

	otherChunks, err := s.ListFogChunks(other.ID)
	if err != nil {
		t.Fatalf("ListFogChunks(other): %v", err)
	}
	if len(otherChunks) != 2 {
		t.Fatalf("other scene chunks = %+v, want both still hidden", otherChunks)
	}
}
