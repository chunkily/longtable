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
	// Also what a retired key does, which is what makes changing the
	// palette safe: the seat reads as unchosen rather than rendering a
	// broken style attribute.
	it('has no colour for a key it does not know', () => {
		expect(identityHex('chartreuse')).toBeNull();
		// A key the palette used to have. Retiring one is safe precisely
		// because it lands here.
		expect(identityHex('emerald')).toBeNull();
	});
});

describe('suggestedColor', () => {
	it('offers the first colour nobody is using', () => {
		expect(suggestedColor([])).toBe('red');
		expect(suggestedColor(['red'])).toBe('orange');
		expect(suggestedColor(['red', 'orange'])).toBe('gold');
	});

	// Seats with no colour don't reserve one.
	it('ignores seats that never chose', () => {
		expect(suggestedColor(['', null, undefined])).toBe('red');
	});

	// A suggestion, not a rule: a seventeenth person at a sixteen-colour
	// table gets a duplicate rather than nothing to click.
	it('falls back to the first colour once they are all taken', () => {
		expect(suggestedColor(IDENTITY_COLORS.map((c) => c.key))).toBe('red');
	});
});

describe('the palette itself', () => {
	it('has no duplicate keys or hexes', () => {
		expect(new Set(IDENTITY_COLORS.map((c) => c.key)).size).toBe(IDENTITY_COLORS.length);
		expect(new Set(IDENTITY_COLORS.map((c) => c.hex)).size).toBe(IDENTITY_COLORS.length);
	});

	// There used to be a test here asserting the palette dodged the
	// colours the canvas speaks in — amber for the erase highlight, sky
	// blue for measuring, red for the fog preview. It was removed with
	// the constraint, deliberately: six colours meant two people matched
	// by the fourth arrival, and none of those clashes is ambiguous on
	// screen. Honey Gold and Blood Red are in the palette *because* that
	// rule was dropped, so a test enforcing it would now be enforcing a
	// decision nobody holds.

	// Every name is `<modifier> <colour>` and the stored key is the base
	// colour alone. That split is what lets a modifier be reworded
	// without touching a single stored seat — and what makes a second
	// green a key collision rather than a naming choice, which is why
	// Swamp Olive is an olive.
	it('keys every colour on the base colour its name ends with', () => {
		for (const { key, label } of IDENTITY_COLORS) {
			const words = label.split(' ');
			expect(words).toHaveLength(2);
			expect(key).toBe(words[1].toLowerCase());
		}
	});
});
