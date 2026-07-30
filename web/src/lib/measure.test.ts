import { describe, expect, it } from 'vitest';
import { cellAt, cellCentre, feetBetween, measureLabel, squaresBetween } from './measure';

describe('cellAt', () => {
	it('floors a world point onto its grid cell', () => {
		expect(cellAt({ x: 0, y: 0 }, 70)).toEqual({ x: 0, y: 0 });
		expect(cellAt({ x: 69.9, y: 140.1 }, 70)).toEqual({ x: 0, y: 2 });
	});

	it('keeps negative coordinates on the cell they fall in', () => {
		// -1 is left of the origin, so it belongs to cell -1, not cell 0.
		expect(cellAt({ x: -1, y: -70 }, 70)).toEqual({ x: -1, y: -1 });
	});
});

describe('cellCentre', () => {
	it('returns the middle of the cell in world space', () => {
		expect(cellCentre({ x: 0, y: 0 }, 70)).toEqual({ x: 35, y: 35 });
		expect(cellCentre({ x: 2, y: -1 }, 70)).toEqual({ x: 175, y: -35 });
	});
});

describe('squaresBetween', () => {
	const origin = { x: 0, y: 0 };

	it('is zero within the same cell', () => {
		expect(squaresBetween(origin, origin)).toBe(0);
	});

	it('counts straight moves one square each', () => {
		expect(squaresBetween(origin, { x: 4, y: 0 })).toBe(4);
		expect(squaresBetween(origin, { x: 0, y: 3 })).toBe(3);
	});

	it('alternates the cost of diagonal steps: 1, 2, 1, 2', () => {
		expect(squaresBetween(origin, { x: 1, y: 1 })).toBe(1);
		expect(squaresBetween(origin, { x: 2, y: 2 })).toBe(3);
		expect(squaresBetween(origin, { x: 3, y: 3 })).toBe(4);
		expect(squaresBetween(origin, { x: 4, y: 4 })).toBe(6);
	});

	it('adds the straight remainder to the diagonal part', () => {
		// 2 diagonals (1 + 2) then 3 straight steps.
		expect(squaresBetween(origin, { x: 5, y: 2 })).toBe(6);
	});

	it('is the same in every direction', () => {
		expect(squaresBetween(origin, { x: -5, y: -2 })).toBe(6);
		expect(squaresBetween({ x: 5, y: 2 }, origin)).toBe(6);
		expect(squaresBetween({ x: -3, y: 4 }, { x: 2, y: 2 })).toBe(6);
	});
});

describe('feetBetween', () => {
	it('reports 5ft per square', () => {
		expect(feetBetween({ x: 0, y: 0 }, { x: 4, y: 4 })).toBe(30);
	});
});

describe('measureLabel', () => {
	it('measures between the cells the two points fall in', () => {
		// Both points are inside their cell rather than on a corner: what
		// gets measured is cell (0,0) to cell (2,2) — 3 squares.
		expect(measureLabel({ x: 10, y: 10 }, { x: 150, y: 155 }, 70)).toBe('15 ft');
	});

	it('reads zero while the drag has not left the starting cell', () => {
		expect(measureLabel({ x: 10, y: 10 }, { x: 60, y: 60 }, 70)).toBe('0 ft');
	});
});
