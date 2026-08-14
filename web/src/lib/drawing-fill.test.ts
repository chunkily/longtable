import { describe, expect, it } from 'vitest';
import { FILL_ALPHA, fillFor } from './drawing-fill';

describe('fillFor', () => {
	it('turns each palette colour into the same hue at the fill alpha', () => {
		expect(fillFor('#cc0000')).toBe(`rgba(204, 0, 0, ${FILL_ALPHA})`);
		expect(fillFor('#008000')).toBe(`rgba(0, 128, 0, ${FILL_ALPHA})`);
		expect(fillFor('#0033cc')).toBe(`rgba(0, 51, 204, ${FILL_ALPHA})`);
	});

	it('keeps black black rather than reading an empty channel as absent', () => {
		expect(fillFor('#000000')).toBe(`rgba(0, 0, 0, ${FILL_ALPHA})`);
	});

	it('expands three-digit hex by doubling each digit, not padding it', () => {
		// #abc is #aabbcc. Padding with zeroes would give (160, 176, 192),
		// which is a different, entirely plausible-looking colour.
		expect(fillFor('#abc')).toBe(`rgba(170, 187, 204, ${FILL_ALPHA})`);
	});

	it('reads upper case and surrounding space', () => {
		expect(fillFor('  #CC0000 ')).toBe(`rgba(204, 0, 0, ${FILL_ALPHA})`);
	});

	it('hands back anything it cannot parse, rather than guessing', () => {
		// Opaque and obviously wrong beats quietly painting a fallback.
		expect(fillFor('red')).toBe('red');
		expect(fillFor('#12345')).toBe('#12345');
		expect(fillFor('')).toBe('');
	});
});
