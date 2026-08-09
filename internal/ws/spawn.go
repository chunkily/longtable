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

	// Footprints rather than corners: a 2×2 token standing at (3,3)
	// covers (4,4) too, and a monkey dropped there would be half under it.
	taken := make([]cell, 0, len(existing)+count)
	sizes := make([]cell, 0, len(existing)+count)
	for _, t := range existing {
		taken = append(taken, cell{X: t.X, Y: t.Y})
		sizes = append(sizes, cell{X: t.Width, Y: t.Height})
	}

	free := func(c cell) bool {
		for i, other := range taken {
			if overlaps(c, w, h, other, sizes[i].X, sizes[i].Y) {
				return false
			}
		}
		return true
	}

	out := make([]cell, 0, count)
	for radius := 0; radius <= maxSpawnRadius && len(out) < count; radius++ {
		for _, c := range ring(originX, originY, radius) {
			if len(out) == count {
				break
			}
			if !free(c) {
				continue
			}
			out = append(out, c)
			taken = append(taken, c)
			sizes = append(sizes, cell{X: w, Y: h})
		}
	}

	// The neighbourhood was full to maxSpawnRadius. Better to stack the
	// remainder on the spawn square than to return fewer tokens than were
	// asked for — the count is what the user typed, and a token they can
	// drag off a pile still exists.
	for len(out) < count {
		out = append(out, cell{X: originX, Y: originY})
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
