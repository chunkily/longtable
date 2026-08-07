/**
 * The arithmetic behind a two-finger pinch on the map.
 *
 * Separated from the Konva handler because a pinch can't be driven from
 * Playwright's ordinary API — `page.touchscreen` is single-touch, so a
 * real two-finger gesture needs raw CDP. Keeping the maths here means the
 * part that has to be *correct* is the part that's cheap to test, and
 * what's left in the component is thin enough to check by hand on a
 * tablet.
 */

export interface Point {
	x: number;
	y: number;
}

/** How far apart two touches are, in screen pixels. */
export function touchDistance(a: Point, b: Point): number {
	return Math.hypot(b.x - a.x, b.y - a.y);
}

/** The point halfway between two touches, in screen pixels. */
export function touchCentre(a: Point, b: Point): Point {
	return { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };
}

export interface PinchStep {
	/** Stage scale before this step. */
	scale: number;
	/** Stage translation before this step, in screen pixels. */
	position: Point;
	/** Midpoint between the touches at the previous step. */
	from: Point;
	/** Midpoint between the touches now. */
	to: Point;
	/** Distance between the touches now, over the distance at the previous step. */
	ratio: number;
	minScale: number;
	maxScale: number;
}

export interface PinchResult {
	scale: number;
	position: Point;
}

/**
 * One step of a pinch: the world point that was under `from` is placed
 * under `to`, at a scale multiplied by `ratio` and clamped to the same
 * bounds the mouse wheel respects.
 *
 * Anchoring on the *previous* midpoint rather than the current one is
 * what makes two-finger panning fall out for free. Anchor on the current
 * midpoint and the algebra cancels — the world point under the fingers is
 * by definition already under the fingers, so sliding both hands across
 * the glass would scale nothing and move nothing, which feels broken on a
 * device where dragging is the main verb.
 *
 * A non-finite or non-positive `ratio` is treated as 1 rather than
 * rejected: it means the previous distance was zero (both touches landing
 * on the same pixel, which happens on the first move of a fast gesture),
 * and the honest answer there is "don't scale yet" rather than an
 * exception thrown out of a touch handler.
 */
export function pinchStep(step: PinchStep): PinchResult {
	const { scale, position, from, to, minScale, maxScale } = step;
	const ratio = Number.isFinite(step.ratio) && step.ratio > 0 ? step.ratio : 1;

	const next = Math.min(maxScale, Math.max(minScale, scale * ratio));

	// The world point under `from`, before anything moves.
	const worldX = (from.x - position.x) / scale;
	const worldY = (from.y - position.y) / scale;

	return {
		scale: next,
		// Put that same world point under `to` at the new scale. When the
		// scale was clamped this still holds — the anchor is computed
		// against whatever scale actually ends up applied, so hitting a
		// limit stops the zoom without letting the map slide.
		position: { x: to.x - worldX * next, y: to.y - worldY * next }
	};
}
