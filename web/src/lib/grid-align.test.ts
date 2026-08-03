import { describe, expect, it } from 'vitest';
import { paddingForOrigin } from './grid-align';

describe('paddingForOrigin', () => {
	it('pads nothing when the art already starts on a grid line', () => {
		expect(paddingForOrigin(70, 0)).toBe(0);
		// A whole square in is still on a line.
		expect(paddingForOrigin(70, 70)).toBe(0);
	});

	it('pads the rest of the square, rather than cropping back to the line', () => {
		// Squares starting 12px in move right by 58, not left by 12 — the
		// 12px strip of art is kept.
		expect(paddingForOrigin(70, 12)).toBe(58);
		expect(paddingForOrigin(64, 1)).toBe(63);
	});

	// The aligner lets the overlay be dragged past a boundary in either
	// direction, and `%` in JavaScript keeps the sign of its left operand,
	// so both of these would otherwise come back negative — which the
	// server reads as a crop.
	it('never returns a negative pad', () => {
		expect(paddingForOrigin(70, 82)).toBe(58);
		expect(paddingForOrigin(70, -12)).toBe(12);
		expect(paddingForOrigin(70, -82)).toBe(12);
	});

	it('rounds a dragged origin to whole pixels', () => {
		expect(paddingForOrigin(70, 12.4)).toBe(58);
		expect(paddingForOrigin(70, 11.6)).toBe(58);
	});

	it('is zero for a grid size that never got measured', () => {
		expect(paddingForOrigin(0, 12)).toBe(0);
		expect(paddingForOrigin(Number.NaN, 12)).toBe(0);
	});
});
