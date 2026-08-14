// What a filled shape's interior is painted with.
//
// Kept out of the canvas because turning `#cc0000` into an `rgba(...)`
// is the kind of thing that is wrong in a way no screenshot shows: a
// mis-parsed channel gives a plausible colour, just not the one on the
// swatch that was clicked.

/**
 * How opaque a shape's fill is.
 *
 * Well short of solid, because a drawing sits over map art that people
 * are still reading — a shaded room should say "this bit" without
 * hiding the furniture in it. Deliberately stronger than the area
 * templates' 0.18 (`TEMPLATE_FILL` in game-canvas.svelte), which are
 * transient and drawn over the top of whatever is already there; a
 * drawing is a deliberate, persistent mark and reads as an accident at
 * that weight.
 *
 * The stroke stays fully opaque. Fading both would make the whole shape
 * look like a preview of itself, and the outline is what gives a shaded
 * area an edge.
 */
export const FILL_ALPHA = 0.3;

/**
 * `color` as a fill: the same hue, at FILL_ALPHA.
 *
 * Accepts `#rgb` and `#rrggbb`, which is every colour this app produces
 * — the four palette swatches and the server's default. Anything else
 * comes back untouched, and therefore opaque: that can only happen if a
 * colour arrived from somewhere new, and a fill that is obviously wrong
 * is easier to chase than one quietly painted in a fallback grey.
 */
export function fillFor(color: string): string {
	const hex = color.trim();
	if (!/^#([0-9a-f]{3}|[0-9a-f]{6})$/i.test(hex)) return color;

	const digits = hex.slice(1);
	// `#abc` is `#aabbcc` — each digit doubled, not padded with zeroes.
	const pairs =
		digits.length === 3
			? [...digits].map((d) => d + d)
			: [digits.slice(0, 2), digits.slice(2, 4), digits.slice(4, 6)];

	const [r, g, b] = pairs.map((pair) => parseInt(pair, 16));
	return `rgba(${r}, ${g}, ${b}, ${FILL_ALPHA})`;
}
