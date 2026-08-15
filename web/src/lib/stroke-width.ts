// The stroke widths the draw strip offers.

import { DRAWING_STROKE_WIDTH } from './drawing-hit';

/**
 * The server's clamp, mirrored from `minDrawingStrokeWidth` and
 * `maxDrawingStrokeWidth` in internal/ws/hub.go. Nothing reads these at
 * runtime — they exist so the test beside this file can catch a choice
 * the server would quietly change on the way through, which draws a
 * stroke that isn't the one the button offered and says nothing.
 */
export const MIN_STROKE_WIDTH = 1;
export const MAX_STROKE_WIDTH = 32;

export interface StrokeWidthChoice {
	label: string;
	/** World pixels, the units a drawing's `strokeWidth` is stored in. */
	value: number;
	/**
	 * Height of the bar that stands for this width, in *screen* pixels —
	 * a shrunken picture of the stroke rather than the stroke itself,
	 * since 16px of bar in a 32px button is a black slab. Drawn short on
	 * the strip's button and long in the picker's rows; only the height
	 * carries the meaning.
	 */
	bar: number;
}

/**
 * Three named sizes rather than the continuous slider this item's title
 * asked for, which is the reading its own later note settled on. Every
 * other setting on that strip is a row of discrete buttons, draw's strip
 * is the first one to scroll on a phone, and a width between two widths
 * isn't a decision anyone at a table wants to make. They sit behind one
 * button rather than on the strip itself — see
 * components/stroke-width-picker.svelte.
 *
 * `Thin` is DRAWING_STROKE_WIDTH, so a browser that never touches the
 * control draws exactly what every stroke made before it existed did —
 * and the strip opens with a button already pressed.
 */
export const STROKE_WIDTH_CHOICES: StrokeWidthChoice[] = [
	{ label: 'Thin', value: DRAWING_STROKE_WIDTH, bar: 1 },
	{ label: 'Medium', value: 8, bar: 3 },
	{ label: 'Thick', value: 16, bar: 6 }
];
