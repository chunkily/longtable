// How big the map being aligned is drawn, and what that does to the
// numbers describing its grid.
//
// This is two lines of arithmetic and it lives here rather than in
// `grid-aligner.svelte` for the reason that component's own doc comment
// gives: everything there is in *natural* image pixels, the preview is
// scaled, and mixing the two up is the easiest bug in the file to
// introduce. Arithmetic in a `.ts` beside a test is the same trade
// `pinch.ts` and `pan.ts` already made — and it is the only way to check
// this at all, since the alternative needs a browser that composites
// frames.

/**
 * Fit means "however wide the frame is"; a number means that many screen
 * pixels per image pixel, so 1 is native scale.
 */
export type Zoom = 'fit' | number;

/**
 * The width the frame around the image is asked to be.
 *
 * Fit answers `'100%'` and hands the job to CSS rather than to a
 * measurement. That matters: a width computed from a measured width is a
 * feedback loop, because the number that comes out changes the layout the
 * number went in from — a scrollbar appearing is enough to start it
 * oscillating. Letting CSS resolve the percentage keeps the loop from
 * existing rather than damping it.
 */
export function frameWidth(zoom: Zoom, naturalWidth: number): string {
	if (zoom === 'fit' || !(naturalWidth > 0) || !(zoom > 0)) return '100%';
	return `${naturalWidth * zoom}px`;
}

/**
 * Screen pixels per image pixel — the number every length in the overlay
 * is multiplied by, and the one the drag divides back out.
 *
 * **A numeric zoom is its own scale, and is not measured.** The frame is
 * set to `naturalWidth * zoom` and nothing narrows it, so at 200% the
 * scale is exactly 2 the moment the button is pressed. Measuring instead
 * would mean reading a width that the browser has not laid out yet: the
 * observer behind `displayWidth` doesn't deliver until the next frame, so
 * for that frame the grid is drawn at the *previous* zoom's scale, over
 * art that has already resized. That is a visible jump, and an e2e test
 * caught it.
 *
 * Fit has no such number and has to be measured, because what it comes
 * out as isn't what it asked for — a scrollbar takes width out of a
 * `100%` without anything else changing.
 *
 * Falls back to 1 rather than 0 or NaN whenever a figure is missing: a 0
 * collapses the grid into nothing and a NaN drops the CSS declaration
 * silently, where 1 draws an unscaled grid over an unmeasured image —
 * visibly wrong for one frame rather than invisibly broken.
 */
export function previewScale(zoom: Zoom, naturalWidth: number, displayWidth: number): number {
	if (zoom !== 'fit' && zoom > 0) return zoom;
	if (!(naturalWidth > 0) || !(displayWidth > 0)) return 1;
	return displayWidth / naturalWidth;
}
