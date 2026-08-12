package store

import (
	"database/sql"
	"errors"
)

// Fog is stored as the set of *hidden* cells, not revealed ones, and
// packed 32 cells to an integer.
//
// Both halves of that follow from the same fact: a scene starts fully
// revealed, and most scenes stay mostly revealed. Storing the hidden set
// makes the common state the one that costs nothing — a fresh scene is
// zero rows, and "reveal everything" is a DELETE rather than an insert
// per cell. It used to be the other way round, which meant a new scene
// had to materialise a row for every cell in its bounds just to look
// normal, and was capped at 40,000 cells because of it.
//
// Packing is what keeps a *deliberately* fogged scene cheap too. A
// 200x200-cell map fully covered is 40,000 hidden cells but only 1,400
// rows here, and the same 32x saving applies to the payload every client
// receives — which is why the chunks go over the wire in this shape
// rather than being expanded back into cells server-side.

// FogCell is a single grid square, which is what the client paints in
// and so what the fog commands carry. Storage and events both speak
// FogChunk; this is only the input side.
type FogCell struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// FogChunk is 32 horizontally adjacent cells of one row: bit n of Mask
// is the cell at x = ChunkX*32 + n, set when that cell is hidden. A
// chunk that isn't stored is 32 revealed cells.
type FogChunk struct {
	Y      int    `json:"y"`
	ChunkX int    `json:"chunkX"`
	Mask   uint32 `json:"mask"`
}

// FogChunkWidth is how many cells one chunk covers. Exported because the
// client packs and unpacks the same layout and the two must agree.
const FogChunkWidth = 32

type chunkKey struct {
	y      int
	chunkX int
}

// chunkFor locates a cell: which chunk column holds it, and which bit
// within that chunk is it.
//
// Shift and mask rather than / and %, because Go's both truncate toward
// zero: x=-1 would divide to chunk 0 and take bit 1, colliding with x=1
// in the same chunk at the same bit. An arithmetic shift floors, so
// x=-1 lands in chunk -1 at bit 31, where it belongs. This is reachable
// — the grid is infinite and a GM can drag fog left of or above the
// map's origin. JavaScript's >> and & do the same thing on int32, which
// is what lets the client pack chunks the server will agree with.
func chunkFor(x int) (chunkX int, bit uint32) {
	return x >> 5, 1 << uint(x&31)
}

// groupCells collapses cells into the bits they set within each chunk,
// so an operation touching n cells costs one query per *chunk* rather
// than per cell. A rectangle drag — the only way fog is painted — is
// contiguous, so this is usually a handful of chunks however big it is.
func groupCells(cells []FogCell) map[chunkKey]uint32 {
	groups := make(map[chunkKey]uint32, len(cells)/FogChunkWidth+1)
	for _, c := range cells {
		chunkX, bit := chunkFor(c.X)
		groups[chunkKey{y: c.Y, chunkX: chunkX}] |= bit
	}
	return groups
}

// HideCells covers cells, and RevealCells uncovers them. Both are
// idempotent — a cell already in the target state is left alone — which
// is what lets the rectangle tool send every cell in its box without
// tracking which ones actually needed changing.
//
// Both return only the chunks whose mask actually changed, with their
// new value, which is exactly what the room needs broadcasting: a drag
// that changed nothing broadcasts nothing.
func (s *Store) HideCells(sceneID string, cells []FogCell) ([]FogChunk, error) {
	return s.applyFog(sceneID, cells, func(current, bits uint32) uint32 { return current | bits })
}

func (s *Store) RevealCells(sceneID string, cells []FogCell) ([]FogChunk, error) {
	return s.applyFog(sceneID, cells, func(current, bits uint32) uint32 { return current &^ bits })
}

func (s *Store) applyFog(
	sceneID string,
	cells []FogCell,
	apply func(current, bits uint32) uint32,
) ([]FogChunk, error) {
	groups := groupCells(cells)
	if len(groups) == 0 {
		return nil, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	changed := make([]FogChunk, 0, len(groups))
	for key, bits := range groups {
		var stored int64
		err := tx.QueryRow(
			`SELECT mask FROM fog_mask WHERE scene_id = ? AND cell_y = ? AND chunk_x = ?`,
			sceneID, key.y, key.chunkX,
		).Scan(&stored)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}

		current := uint32(stored)
		next := apply(current, bits)
		if next == current {
			continue
		}

		// A chunk with nothing hidden is deleted rather than kept at
		// zero, so "no row" stays the only spelling of "all revealed" —
		// two spellings would mean every read had to handle both.
		if next == 0 {
			_, err = tx.Exec(
				`DELETE FROM fog_mask WHERE scene_id = ? AND cell_y = ? AND chunk_x = ?`,
				sceneID, key.y, key.chunkX,
			)
		} else {
			_, err = tx.Exec(
				`INSERT INTO fog_mask (scene_id, cell_y, chunk_x, mask) VALUES (?, ?, ?, ?)
				 ON CONFLICT (scene_id, cell_y, chunk_x) DO UPDATE SET mask = excluded.mask`,
				sceneID, key.y, key.chunkX, int64(next),
			)
		}
		if err != nil {
			return nil, err
		}
		changed = append(changed, FogChunk{Y: key.y, ChunkX: key.chunkX, Mask: next})
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return changed, nil
}

// RevealAllCells uncovers a whole scene. It needs no scene bounds and
// can't be too large to do: it only has to describe the chunks that
// currently hold fog, however big the map is. Reveal-all used to be the
// operation with a cap on it, for exactly the reason this one no longer
// needs one.
//
// The returned chunks carry mask 0 — the room is told those chunks are
// now empty rather than being told to forget them, so one merge rule
// covers every fog event.
func (s *Store) RevealAllCells(sceneID string) ([]FogChunk, error) {
	existing, err := s.ListFogChunks(sceneID)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`DELETE FROM fog_mask WHERE scene_id = ?`, sceneID); err != nil {
		return nil, err
	}

	cleared := make([]FogChunk, 0, len(existing))
	for _, c := range existing {
		cleared = append(cleared, FogChunk{Y: c.Y, ChunkX: c.ChunkX, Mask: 0})
	}
	return cleared, nil
}

// HideAllCells covers a whole scene, replacing whatever fog it had with
// the given chunks — the caller works out what "the whole scene" means,
// since only it knows the scene's bounds (see sceneFogChunks in the
// hub, which also caps the size).
//
// The delta returned includes any chunk that existed before and isn't in
// the new set, zeroed. Those are cells outside the scene's bounds — fog
// a GM painted left of or above the origin — and without them a client
// would keep drawing fog the server had just deleted.
func (s *Store) HideAllCells(sceneID string, chunks []FogChunk) ([]FogChunk, error) {
	existing, err := s.ListFogChunks(sceneID)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM fog_mask WHERE scene_id = ?`, sceneID); err != nil {
		return nil, err
	}

	stmt, err := tx.Prepare(
		`INSERT INTO fog_mask (scene_id, cell_y, chunk_x, mask) VALUES (?, ?, ?, ?)`,
	)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	replaced := make(map[chunkKey]struct{}, len(chunks))
	for _, c := range chunks {
		if c.Mask == 0 {
			continue
		}
		if _, err := stmt.Exec(sceneID, c.Y, c.ChunkX, int64(c.Mask)); err != nil {
			return nil, err
		}
		replaced[chunkKey{y: c.Y, chunkX: c.ChunkX}] = struct{}{}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	delta := make([]FogChunk, 0, len(chunks)+len(existing))
	delta = append(delta, chunks...)
	for _, c := range existing {
		if _, kept := replaced[chunkKey{y: c.Y, chunkX: c.ChunkX}]; !kept {
			delta = append(delta, FogChunk{Y: c.Y, ChunkX: c.ChunkX, Mask: 0})
		}
	}
	return delta, nil
}

func (s *Store) ListFogChunks(sceneID string) ([]FogChunk, error) {
	rows, err := s.db.Query(
		`SELECT cell_y, chunk_x, mask FROM fog_mask WHERE scene_id = ?`, sceneID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []FogChunk
	for rows.Next() {
		var c FogChunk
		var mask int64
		if err := rows.Scan(&c.Y, &c.ChunkX, &mask); err != nil {
			return nil, err
		}
		c.Mask = uint32(mask)
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}
