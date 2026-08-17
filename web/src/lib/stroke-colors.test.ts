import { describe, expect, it } from 'vitest';
import {
	DARK_MAP_STROKE_COLORS,
	DEFAULT_STROKE_COLOR,
	LIGHT_MAP_STROKE_COLORS,
	STROKE_COLOR_ROWS
} from './stroke-colors';

const every = STROKE_COLOR_ROWS.flat();

describe('STROKE_COLOR_ROWS', () => {
	// A swatch reads as selected when its value is the active colour, so
	// two swatches sharing one would both ring at once — and the panel
	// would claim two colours are in use.
	it('gives every swatch its own colour', () => {
		const values = every.map((colour) => colour.value);
		expect(new Set(values).size).toBe(values.length);
	});

	// The swatches have no visible text, so the label is the whole of what
	// a screen reader announces and the whole of what a spec can find one
	// by. Two the same and neither can be picked out.
	it('gives every swatch its own name', () => {
		const labels = every.map((colour) => colour.label);
		expect(new Set(labels).size).toBe(labels.length);
	});

	// The value goes straight into a `background-color`, where anything
	// malformed is silently no colour at all — an invisible swatch that
	// still draws in whatever the canvas makes of it.
	it('offers only well-formed hex', () => {
		for (const colour of every) {
			expect(colour.value).toMatch(/^#[0-9a-f]{6}$/);
		}
	});

	// The rows are laid out one above the other and read in columns: white
	// under black, bright red under red. A row of a different length puts
	// every swatch past the gap under the wrong one.
	it('keeps the two rows the same length, so they pair up in columns', () => {
		expect(DARK_MAP_STROKE_COLORS).toHaveLength(LIGHT_MAP_STROKE_COLORS.length);
	});

	// Nothing picks a colour until someone clicks one, so a default that
	// isn't in the palette leaves the panel open with no swatch ringed,
	// and the button wearing a colour it can't show you.
	it('includes the default colour', () => {
		expect(every.map((colour) => colour.value)).toContain(DEFAULT_STROKE_COLOR);
	});
});
