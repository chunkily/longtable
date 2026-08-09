package ws

import (
	"testing"

	"longtable/internal/store"
)

// Where a batch lands is pure arithmetic, so it's tested here rather
// than through the socket — the hub tests only have to prove the handler
// calls this at all.

func at(x, y float64) store.Token {
	return store.Token{X: x, Y: y, Width: 1, Height: 1}
}

func TestSpawnCells_ASingleTokenGoesExactlyWhereItWasAsked(t *testing.T) {
	// Even onto an occupied square. This is the undo of a deletion: the
	// token has to come back on its own square rather than politely
	// beside whatever has parked there since.
	got := spawnCells(3, 4, 1, 1, 1, []store.Token{at(3, 4)})
	if len(got) != 1 || got[0] != (cell{X: 3, Y: 4}) {
		t.Fatalf("spawnCells = %v, want exactly [(3,4)]", got)
	}
}

func TestSpawnCells_ABatchIsABlockOfDistinctSquares(t *testing.T) {
	got := spawnCells(0, 0, 8, 1, 1, nil)
	if len(got) != 8 {
		t.Fatalf("len = %d, want 8", len(got))
	}

	seen := map[cell]bool{}
	for _, c := range got {
		if seen[c] {
			t.Fatalf("%v appears twice — eight monkeys on seven squares is a stack", c)
		}
		seen[c] = true
		// Ring order means eight tokens never reach past the first ring
		// around the origin, which is what makes the result a block rather
		// than a line.
		if c.X < -1 || c.X > 1 || c.Y < -1 || c.Y > 1 {
			t.Errorf("%v is outside the first ring, so the block has holes in it", c)
		}
	}
	if got[0] != (cell{X: 0, Y: 0}) {
		t.Errorf("first cell = %v, want the square that was pointed at", got[0])
	}
}

func TestSpawnCells_SkipsSquaresSomethingIsStandingOn(t *testing.T) {
	occupied := []store.Token{at(0, 0), at(1, 0), at(-1, -1)}
	got := spawnCells(0, 0, 3, 1, 1, occupied)

	for _, c := range got {
		for _, taken := range occupied {
			if c.X == taken.X && c.Y == taken.Y {
				t.Fatalf("%v is already occupied", c)
			}
		}
	}
}

// A 2×2 token standing at (0,0) covers (1,1) as well, so a square that
// merely has no token's *corner* on it can still be under one.
func TestSpawnCells_RespectsFootprintsRatherThanCorners(t *testing.T) {
	big := store.Token{X: 0, Y: 0, Width: 2, Height: 2}
	got := spawnCells(0, 0, 2, 1, 1, []store.Token{big})

	for _, c := range got {
		if c.X >= 0 && c.X < 2 && c.Y >= 0 && c.Y < 2 {
			t.Fatalf("%v is underneath the 2x2 token at the origin", c)
		}
	}
}

// Large tokens have to clear each other too, which the ring search only
// gets right because it tests the whole footprint of each candidate.
func TestSpawnCells_LargeTokensDoNotOverlapEachOther(t *testing.T) {
	got := spawnCells(0, 0, 4, 2, 2, nil)
	for i, a := range got {
		for _, b := range got[i+1:] {
			if overlaps(a, 2, 2, b, 2, 2) {
				t.Fatalf("%v and %v overlap", a, b)
			}
		}
	}
}
