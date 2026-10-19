package ws

import "longtable/internal/store"

// Where a batch of new tokens lands.
//
// Eight conjured monkeys created on one square would be eight tokens
// stacked on top of each other, and getting at what's underneath a stack
// is already the hardest thing to do on the map — so a batch spreads
// outward from the square it was asked for, into squares nothing is
// standing on.
//
// The search is by Chebyshev ring: the spawn square first, then the
// eight around it, then the sixteen around those. That produces a
// roughly square block rather than a line, which is what a group of
// summons wants to be, and it keeps the first token on the square the
// creator actually pointed at whenever that square is free.

// maxSpawnRadius bounds the ring search. Twenty tokens need a radius of
// two on an empty map, so anything near this means the neighbourhood is
// packed — at which point stacking is a better answer than walking the
// whole plane looking for a gap.
const maxSpawnRadius = 32

// cell is a square on the grid. Token coordinates are grid squares, not
// pixels, so these are the same units a token's X/Y are stored in.
type cell struct {
	X, Y float64
}

// spawnCells picks count squares for tokens of w×h squares, starting
// from (originX, originY) and spreading outward past anything standing
// in the way.
//
// A count of one always returns the origin exactly, even when something
// is already there. That case isn't a batch — it's a single token going
// where it was asked to go, including the undo of a deletion, which has
// to put the token back on its own square rather than politely beside
// whatever has parked there since.
func spawnCells(originX, originY float64, count int, w, h float64, existing []store.Token) []cell {
	if count <= 1 {
		return []cell{{X: originX, Y: originY}}
	}
	sizes := make([]cell, count)
	for i := range sizes {
		sizes[i] = cell{X: w, Y: h}
	}
	return spawnFootprints(originX, originY, sizes, existing)
}

// spawnFootprints is spawnCells for tokens that needn't be one size —
// the party, where a Large companion stands beside four Medium
// characters. Each token in turn takes the first free square nearest the
// origin that its own footprint fits, with everything placed before it
// counted as standing there.
//
// For a batch of one size that is exactly what the single walk outward
// spawnCells used to do: a square too crowded for one token stays too
// crowded for the next, so searching again from the middle finds the
// same squares in the same order. Unlike spawnCells it never treats one
// token as special — a party of one still steps off an occupied square,
// because nobody pointed at that square for it.
func spawnFootprints(originX, originY float64, sizes []cell, existing []store.Token) []cell {
	// Footprints rather than corners: a 2×2 token standing at (3,3)
	// covers (4,4) too, and a monkey dropped there would be half under it.
	taken := make([]cell, 0, len(existing)+len(sizes))
	takenSizes := make([]cell, 0, len(existing)+len(sizes))
	for _, t := range existing {
		taken = append(taken, cell{X: t.X, Y: t.Y})
		takenSizes = append(takenSizes, cell{X: t.Width, Y: t.Height})
	}

	free := func(c, size cell) bool {
		for i, other := range taken {
			if overlaps(c, size.X, size.Y, other, takenSizes[i].X, takenSizes[i].Y) {
				return false
			}
		}
		return true
	}

	out := make([]cell, 0, len(sizes))
	for _, size := range sizes {
		// The neighbourhood full to maxSpawnRadius stacks the token on the
		// spawn square rather than dropping it — better than returning
		// fewer tokens than were asked for, since one on a pile can still
		// be dragged off it.
		spot := cell{X: originX, Y: originY}
	search:
		for radius := 0; radius <= maxSpawnRadius; radius++ {
			for _, c := range ring(originX, originY, radius) {
				if free(c, size) {
					spot = c
					break search
				}
			}
		}
		out = append(out, spot)
		taken = append(taken, spot)
		takenSizes = append(takenSizes, size)
	}
	return out
}

// overlaps reports whether two footprints share any square.
func overlaps(a cell, aw, ah float64, b cell, bw, bh float64) bool {
	return a.X < b.X+bw && b.X < a.X+aw && a.Y < b.Y+bh && b.Y < a.Y+ah
}

// ring lists the squares at exactly Chebyshev distance `radius` from the
// origin, clockwise from the top-left corner. Radius zero is the origin
// itself.
//
// The order within a ring decides which way an oddly-shaped block leans
// and nothing else, but it does have to be *deterministic*: the same
// batch created twice should lay out the same way, and a test asserting
// where eight monkeys landed would otherwise be asserting a map
// iteration order.
func ring(originX, originY float64, radius int) []cell {
	if radius == 0 {
		return []cell{{X: originX, Y: originY}}
	}

	r := float64(radius)
	out := make([]cell, 0, 8*radius)
	for dx := -r; dx <= r; dx++ {
		out = append(out, cell{X: originX + dx, Y: originY - r})
	}
	for dy := -r + 1; dy <= r; dy++ {
		out = append(out, cell{X: originX + r, Y: originY + dy})
	}
	for dx := r - 1; dx >= -r; dx-- {
		out = append(out, cell{X: originX + dx, Y: originY + r})
	}
	for dy := r - 1; dy >= -r+1; dy-- {
		out = append(out, cell{X: originX - r, Y: originY + dy})
	}
	return out
}
