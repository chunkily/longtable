import { describe, expect, it } from 'vitest';
import { FOG_CHUNK_WIDTH, fogChunkFor, fogRuns } from './fog';

describe('fogChunkFor', () => {
	it('splits a cell into its chunk column and bit', () => {
		expect(fogChunkFor(0)).toEqual({ chunkX: 0, bit: 0b1 });
		expect(fogChunkFor(3)).toEqual({ chunkX: 0, bit: 0b1000 });
		expect(fogChunkFor(31)).toEqual({ chunkX: 0, bit: 1 << 31 });
		expect(fogChunkFor(32)).toEqual({ chunkX: 1, bit: 0b1 });
		expect(fogChunkFor(33)).toEqual({ chunkX: 1, bit: 0b10 });
	});

	// The grid is infinite, so a GM can drag fog left of or above the
	// map's origin. Dividing instead of shifting would fold x=-1 onto
	// chunk 0 — the same chunk as x=1, and at a bit that collides with a
	// real cell there.
	it('floors negative cells into the chunk below zero instead of folding them onto zero', () => {
		expect(fogChunkFor(-1)).toEqual({ chunkX: -1, bit: 1 << 31 });
		expect(fogChunkFor(-32)).toEqual({ chunkX: -1, bit: 0b1 });
		expect(fogChunkFor(-33)).toEqual({ chunkX: -2, bit: 1 << 31 });

		expect(fogChunkFor(-1).chunkX).not.toBe(fogChunkFor(1).chunkX);
	});
});

describe('fogRuns', () => {
	it('has nothing to draw for a scene with no fog on it', () => {
		expect(fogRuns([])).toEqual([]);
	});

	it('turns an unbroken span of bits into one run rather than one per cell', () => {
		expect(fogRuns([{ y: 2, chunkX: 0, mask: 0b1110 }])).toEqual([{ x: 1, y: 2, length: 3 }]);
	});

	it('breaks a run wherever a cell is revealed', () => {
		expect(fogRuns([{ y: 0, chunkX: 0, mask: 0b1011 }])).toEqual([
			{ x: 0, y: 0, length: 2 },
			{ x: 3, y: 0, length: 1 }
		]);
	});

	it('offsets runs by the chunk they came from', () => {
		expect(fogRuns([{ y: 1, chunkX: 3, mask: 0b11 }])).toEqual([
			{ x: 3 * FOG_CHUNK_WIDTH, y: 1, length: 2 }
		]);
	});

	// JavaScript's bitwise operators work on signed int32, so bit 31
	// makes both `1 << 31` and the AND against it negative. Testing the
	// AND for truthiness or `> 0` would drop the last cell of every
	// chunk — a one-cell gap down the right edge of every 32nd column.
	it('includes the top bit, which is negative in JavaScript', () => {
		expect(fogRuns([{ y: 0, chunkX: 0, mask: 1 << 31 }])).toEqual([{ x: 31, y: 0, length: 1 }]);
	});

	it('runs a fully covered chunk edge to edge', () => {
		expect(fogRuns([{ y: 0, chunkX: 0, mask: ~0 }])).toEqual([
			{ x: 0, y: 0, length: FOG_CHUNK_WIDTH }
		]);
	});

	// Two full chunks side by side give two runs that meet exactly rather
	// than one long one. That's deliberate: the caller draws every run
	// into a single path and fills it once, so abutting rectangles union
	// with no seam and merging across the boundary would buy nothing.
	it('stops runs at chunk boundaries even when the fog carries on', () => {
		expect(
			fogRuns([
				{ y: 0, chunkX: 0, mask: ~0 },
				{ y: 0, chunkX: 1, mask: ~0 }
			])
		).toEqual([
			{ x: 0, y: 0, length: FOG_CHUNK_WIDTH },
			{ x: FOG_CHUNK_WIDTH, y: 0, length: FOG_CHUNK_WIDTH }
		]);
	});

	it('keeps rows apart', () => {
		expect(
			fogRuns([
				{ y: 0, chunkX: 0, mask: 0b1 },
				{ y: 5, chunkX: 0, mask: 0b1 }
			])
		).toEqual([
			{ x: 0, y: 0, length: 1 },
			{ x: 0, y: 5, length: 1 }
		]);
	});

	it('draws fog painted left of the origin where it was painted', () => {
		const { chunkX, bit } = fogChunkFor(-1);
		expect(fogRuns([{ y: 0, chunkX, mask: bit }])).toEqual([{ x: -1, y: 0, length: 1 }]);
	});
});
