// The colours a drawing can be, and the two rows the draw strip's
// colour picker lays them out in (components/stroke-color-picker.svelte).
//
// The first row was picked against light maps and mostly disappears on a
// dark one — black most of all, but the saturated red, green and blue
// aren't much better. The second row is the same four hues taken bright
// enough to read on dark art, with white standing in for black.
//
// Pastel tints were tried for that row first and rejected: legible, but
// washed out, and not obviously the same colour family as the swatch
// above them at a glance, which is the whole of how a second row is
// meant to be read.
//
// # Both rows, always
//
// Neither row is hidden or swapped for the other, and nothing here looks
// at the app's theme. The theme says what the page is wearing; a map is
// a picture, and a dark battle map under a light UI is exactly the case
// this exists for. Detection would also have to be per-scene and would
// still be wrong on a map that is half torchlight and half cave. So both
// rows sit there and the artist picks — the same trade the identity
// palette makes when it leaves contrast to whoever chooses (see
// identity-color.ts).

export interface StrokeColor {
	/** The swatch's accessible name — it has no visible text. */
	label: string;
	/** Hex, stored as the drawing's `color` and handed to Konva's stroke. */
	value: string;
}

/** For light map art, and what a browser opens with. */
export const LIGHT_MAP_STROKE_COLORS: StrokeColor[] = [
	{ label: 'Black', value: '#000000' },
	{ label: 'Red', value: '#cc0000' },
	{ label: 'Green', value: '#008000' },
	{ label: 'Blue', value: '#0033cc' }
];

/** For dark map art. Paired with the row above, hue for hue, in order. */
export const DARK_MAP_STROKE_COLORS: StrokeColor[] = [
	{ label: 'White', value: '#ffffff' },
	{ label: 'Bright red', value: '#ff3b30' },
	{ label: 'Bright green', value: '#00e676' },
	{ label: 'Bright blue', value: '#2979ff' }
];

/**
 * What the picker lays out, in order. Rows rather than one long row: the
 * pairing is the point, and eight swatches abreast would lose it.
 */
export const STROKE_COLOR_ROWS: StrokeColor[][] = [LIGHT_MAP_STROKE_COLORS, DARK_MAP_STROKE_COLORS];

/**
 * The colour a browser draws in until someone picks another. Black, so a
 * browser that never opens the picker draws what every stroke made before
 * the second row existed did — and the panel opens with a swatch already
 * ringed.
 */
export const DEFAULT_STROKE_COLOR = LIGHT_MAP_STROKE_COLORS[0].value;
