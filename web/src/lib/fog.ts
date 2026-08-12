// The fog wire format, and getting drawable shapes back out of it.
//
// Fog is the set of *hidden* cells — the inverse of what it used to be —
// packed 32 cells to an integer. A scene with no fog on it is an empty
// list rather than a full one, which is why a new scene needs nothing
// done to it to come up revealed, and why a fully covered 200x200-cell
// map syncs as 1,400 numbers instead of 40,000 pairs.
//
// The server stores and sends exactly this shape; see
// internal/store/fog.go, which is the other half of this file and has to
// agree with it bit for bit.

/** Cells per chunk. Must agree with store.FogChunkWidth on the server. */
export const FOG_CHUNK_WIDTH = 32;

/**
 * 32 horizontally adjacent cells of one row: bit n of `mask` is the cell
 * at `x = chunkX * 32 + n`, set when that cell is **hidden**. A chunk
 * that isn't in the list is 32 revealed cells, and a chunk whose mask
 * reaches 0 is dropped rather than kept — "absent" is the only spelling
 * of "revealed" on either side of the wire.
 */
export interface FogChunk {
	y: number;
	chunkX: number;
	mask: number;
}

/** A horizontal strip of hidden cells: `length` cells starting at `x`. */
export interface FogRun {
	x: number;
	y: number;
	length: number;
}

/**
 * The chunk column a cell's x belongs to, and the bit within that chunk.
 *
 * `>>` and `&` rather than `/` and `%` so a negative x floors instead of
 * truncating toward zero: x=-1 belongs in chunk -1 at bit 31, not chunk
 * 0 at bit 1 where it would collide with x=1. The grid is infinite and
 * fog can be painted left of or above the map's origin. Go's `chunkFor`
 * does the same thing, which is what lets the two sides agree about a
 * cell nobody thought to test.
 */
export function fogChunkFor(x: number): { chunkX: number; bit: number } {
	return { chunkX: x >> 5, bit: 1 << (x & 31) };
}

/**
 * Unpacks chunks into horizontal runs of hidden cells — one run per
 * unbroken span, rather than one per cell, so a covered room is a
 * handful of rectangles instead of hundreds.
 *
 * Runs stop at chunk boundaries even when the fog doesn't: two adjacent
 * full chunks give two runs that meet exactly rather than one long one.
 * That costs nothing, because the caller draws every run into a single
 * path and fills it once — abutting rectangles in one path union
 * cleanly, with no seam between them to be worth eliminating here.
 */
export function fogRuns(chunks: FogChunk[]): FogRun[] {
	const runs: FogRun[] = [];

	for (const chunk of chunks) {
		const baseX = chunk.chunkX * FOG_CHUNK_WIDTH;
		let start = -1;

		for (let bit = 0; bit < FOG_CHUNK_WIDTH; bit++) {
			// Compared against 0 rather than tested for truthiness or
			// positivity: JavaScript's bitwise operators work on *signed*
			// int32, so bit 31 makes both `1 << bit` and the AND negative.
			// `> 0` would silently drop the last cell of every chunk.
			const hidden = (chunk.mask & (1 << bit)) !== 0;

			if (hidden && start === -1) {
				start = bit;
			} else if (!hidden && start !== -1) {
				runs.push({ x: baseX + start, y: chunk.y, length: bit - start });
				start = -1;
			}
		}

		if (start !== -1) {
			runs.push({ x: baseX + start, y: chunk.y, length: FOG_CHUNK_WIDTH - start });
		}
	}

	return runs;
}
