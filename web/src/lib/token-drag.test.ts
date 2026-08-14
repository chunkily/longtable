import { describe, expect, it } from 'vitest';
import { snapTokenCell, tokenCentre, tokenDragPreview } from './token-drag';

const ONE_BY_ONE = { width: 1, height: 1 };

describe('snapTokenCell', () => {
	it('rounds to the nearest grid line rather than flooring', () => {
		// 104 is past the halfway mark of the second square, so the corner
		// is nearest the line at cell 1 even though it sits inside cell 1.
		expect(snapTokenCell({ x: 104, y: 36 }, 70)).toEqual({ x: 1, y: 1 });
		expect(snapTokenCell({ x: 34, y: 34 }, 70)).toEqual({ x: 0, y: 0 });
	});

	it('snaps left and up of the origin the same way', () => {
		expect(snapTokenCell({ x: -36, y: -104 }, 70)).toEqual({ x: -1, y: -1 });
	});
});

describe('tokenCentre', () => {
	it('puts a 1x1 token in the middle of its own cell', () => {
		expect(tokenCentre({ x: 0, y: 0 }, ONE_BY_ONE, 70)).toEqual({ x: 35, y: 35 });
	});

	it('puts a 2x2 token where its four cells meet', () => {
		expect(tokenCentre({ x: 0, y: 0 }, { width: 2, height: 2 }, 70)).toEqual({ x: 70, y: 70 });
	});

	it('handles a token that is wider than it is tall', () => {
		expect(tokenCentre({ x: 1, y: 1 }, { width: 3, height: 1 }, 70)).toEqual({ x: 175, y: 105 });
	});
});

describe('tokenDragPreview', () => {
	const origin = { x: 2, y: 2 };

	it('reads zero and reports no move while still on the starting square', () => {
		const preview = tokenDragPreview(origin, ONE_BY_ONE, { x: 150, y: 148 }, 70);
		expect(preview.cell).toEqual(origin);
		expect(preview.label).toBe('0 ft');
		expect(preview.moved).toBe(false);
	});

	it('measures to the square it will snap to, not to the pointer', () => {
		// 356 rounds to cell 5, three squares across. A raw-position
		// reading would call this 2.94 squares and round the *distance*
		// instead, which is a different number on most drags.
		const preview = tokenDragPreview(origin, ONE_BY_ONE, { x: 356, y: 140 }, 70);
		expect(preview.cell).toEqual({ x: 5, y: 2 });
		expect(preview.label).toBe('15 ft');
		expect(preview.moved).toBe(true);
	});

	it('counts diagonals by the alternating rule, like the ruler does', () => {
		// Two diagonal steps: the first costs 1 square, the second 2.
		const preview = tokenDragPreview(origin, ONE_BY_ONE, { x: 280, y: 280 }, 70);
		expect(preview.cell).toEqual({ x: 4, y: 4 });
		expect(preview.label).toBe('15 ft');
	});

	it('measures a large token by its displacement, not by its bulk', () => {
		// A 3x3 dragged three squares right has travelled 15 ft, the same
		// as a 1x1 would. Its centres are further apart in world space but
		// the cells it is measured between are displaced identically.
		const big = { width: 3, height: 3 };
		const preview = tokenDragPreview(origin, big, { x: 350, y: 140 }, 70);
		expect(preview.label).toBe('15 ft');
		expect(preview.from).toEqual({ x: 245, y: 245 });
		expect(preview.to).toEqual({ x: 455, y: 245 });
	});

	it('hangs the label off the top edge of the destination, clear of the art', () => {
		const preview = tokenDragPreview(origin, { width: 3, height: 3 }, { x: 350, y: 350 }, 70);
		// Centred on the token horizontally, but level with the top of the
		// square it lands on rather than with its middle.
		expect(preview.labelAt).toEqual({ x: 455, y: 350 });
		expect(preview.to.y).toBe(455);
	});

	it('draws the line between centres, so it starts and ends on the token', () => {
		const preview = tokenDragPreview({ x: 0, y: 0 }, ONE_BY_ONE, { x: 140, y: 0 }, 70);
		expect(preview.from).toEqual({ x: 35, y: 35 });
		expect(preview.to).toEqual({ x: 175, y: 35 });
	});

	it('works on a drag back across the origin', () => {
		const preview = tokenDragPreview(origin, ONE_BY_ONE, { x: -70, y: 140 }, 70);
		expect(preview.cell).toEqual({ x: -1, y: 2 });
		expect(preview.label).toBe('15 ft');
	});
});
