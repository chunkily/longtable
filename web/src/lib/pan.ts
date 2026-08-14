/**
 * The arithmetic behind dragging the map with a non-primary mouse
 * button.
 *
 * Two lines of addition, and separated out for the invariant underneath
 * them rather than for the complexity: **a pan is measured in screen
 * pixels against a position sampled once, at the press.** Both halves of
 * that are load-bearing, and getting either wrong produces a map that
 * moves — just not with the pointer, which is the hardest kind of bug to
 * see in a screenshot.
 *
 * *Screen pixels*, because the obvious source of a pointer position on a
 * Konva stage is `getRelativePointerPosition()`, which applies the
 * inverse of the stage transform. Feeding that back into the stage's own
 * translation makes each frame's delta depend on the translation the
 * previous frame just set, and the map accelerates away under the hand.
 *
 * *Sampled once*, because anchoring every step to where the drag started
 * — rather than accumulating deltas between consecutive pointer samples
 * — makes the result depend only on where the pointer is now. A gesture
 * delivered as fifty small moves then lands in exactly the same place as
 * the same gesture delivered as one, which is what `pan.test.ts` pins.
 */

// The same screen-pixel point the pinch maths uses. Imported rather than
// declared again so the two gestures can't drift into disagreeing about
// what a point on the glass is.
import type { Point } from './pinch';

export type { Point };

/**
 * The mouse buttons that drag the map: right and middle.
 *
 * Left is deliberately absent. It belongs to whatever tool is active,
 * and to Konva's own stage drag when none is — which is the whole reason
 * these two exist, since they are the buttons no tool has a use for and
 * can therefore pan whatever is selected.
 *
 * `MouseEvent.button` numbering, not the `buttons` bitmask — the two
 * spellings disagree about these very buttons. In `button`, 1 is middle
 * and 2 is right; in the `buttons` mask, 1 is *left* and 2 is right.
 * Reading a mask through this would give a pan that fires on the left
 * button and never on the middle one.
 */
export function isPanButton(button: number): boolean {
	return button === 2 || button === 1;
}

export interface PanStep {
	/** The stage's translation when the drag began, in screen pixels. */
	origin: Point;
	/** Where the pointer was when the drag began, in screen pixels. */
	from: Point;
	/** Where the pointer is now, in screen pixels. */
	to: Point;
}

/**
 * The stage translation that puts the world point grabbed at `from`
 * under the pointer at `to`.
 *
 * Scale doesn't appear here, and that's correct rather than an omission:
 * the stage's translation is applied *after* its scale, so both are
 * screen-space quantities and a pointer that moved 100px right wants the
 * translation 100px right at every zoom level. This is exactly what
 * makes a pan cheaper than a zoom to re-render — nothing sized in screen
 * pixels has changed size, so only the grid has to be recomputed for the
 * newly visible region.
 */
export function panStep({ origin, from, to }: PanStep): Point {
	return { x: origin.x + (to.x - from.x), y: origin.y + (to.y - from.y) };
}
