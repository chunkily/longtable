import { describe, expect, it } from 'vitest';
import { IDENTITY_COLORS, identityHex, suggestedColor } from './identity-color';

describe('identityHex', () => {
	it('resolves a stored key to the colour it is painted in', () => {
		expect(identityHex('violet')).toBe('#8b5cf6');
	});

	// Every seat made before colours existed carries an empty key, and
	// they should look the way they always did rather than all turning
	// the first palette colour on the same afternoon.
	it('has no colour for a seat that never chose one', () => {
		expect(identityHex('')).toBeNull();
		expect(identityHex(null)).toBeNull();
		expect(identityHex(undefined)).toBeNull();
	});

	// The server refuses anything outside the palette, so this only
	// happens if the two lists drift — and rendering nothing is the safe
	// end of that, since the value's destination is a style attribute.
	it('has no colour for a key it does not know', () => {
		expect(identityHex('crimson')).toBeNull();
	});
});

describe('suggestedColor', () => {
	it('offers the first colour nobody is using', () => {
		expect(suggestedColor([])).toBe('violet');
		expect(suggestedColor(['violet'])).toBe('indigo');
		expect(suggestedColor(['violet', 'indigo'])).toBe('teal');
	});

	// Seats with no colour don't reserve one.
	it('ignores seats that never chose', () => {
		expect(suggestedColor(['', null, undefined])).toBe('violet');
	});

	// A suggestion, not a rule: a seventh person at a six-colour table
	// gets a duplicate rather than nothing to click.
	it('falls back to the first colour once they are all taken', () => {
		expect(suggestedColor(IDENTITY_COLORS.map((c) => c.key))).toBe('violet');
	});
});

describe('the palette itself', () => {
	it('has no duplicate keys or hexes', () => {
		expect(new Set(IDENTITY_COLORS.map((c) => c.key)).size).toBe(IDENTITY_COLORS.length);
		expect(new Set(IDENTITY_COLORS.map((c) => c.hex)).size).toBe(IDENTITY_COLORS.length);
	});

	// The canvas already speaks in amber (the erase highlight), sky blue
	// (measuring) and red (the fog-hide preview). An identity colour that
	// collided with one of those would say something about the map rather
	// than about a person.
	it('avoids the colours the canvas already uses', () => {
		const spokenFor = ['#f59e0b', '#0ea5e9', '#dc2626'];
		for (const { hex } of IDENTITY_COLORS) {
			expect(spokenFor).not.toContain(hex);
		}
	});
});
