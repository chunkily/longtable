// Where a token being dragged would land, and how far that is.
//
// Kept out of the canvas so the rule can be tested without a Konva
// stage, and — more to the point — so the square the *preview* promises
// and the square the drop actually sends are worked out by the same
// function. Before this existed the rounding was one expression written
// once in `dragend`; a preview that computed it a second time is exactly
// how a preview starts lying.

import { cellCentre, feetBetween, type Cell } from './measure';
import type { DrawingPoint } from './room.svelte';

/** Just enough of a Token to place it: how many cells it covers. */
export interface TokenSize {
	width: number;
	height: number;
}

export interface TokenDragPreview {
	/** The cell the token lands on if it's dropped now. */
	cell: Cell;
	/** World-space centre of the token where the drag began. */
	from: DrawingPoint;
	/** World-space centre of where it would land. */
	to: DrawingPoint;
	/**
	 * Where the badge hangs from: the top edge of the destination square,
	 * not its centre. Anchored to the centre it lands *on* the art — on a
	 * 1x1 that's a clipped corner, on a 3x3 it's a label sitting in the
	 * middle of the creature it's describing.
	 */
	labelAt: DrawingPoint;
	/** The distance, worded for the label above the line. */
	label: string;
	/** Whether it has actually left the square it was picked up from. */
	moved: boolean;
}

/**
 * The cell a dragged token snaps to. Rounded, not floored: a token's
 * stored position is the cell its *top-left corner* occupies, so what's
 * being asked is which grid line the corner is nearest — not which cell
 * the corner happens to be sitting inside, which `cellAt` answers and
 * which would drop every token a square up and left of where it looks.
 */
export function snapTokenCell(position: DrawingPoint, gridSize: number): Cell {
	return { x: Math.round(position.x / gridSize), y: Math.round(position.y / gridSize) };
}

/**
 * The world-space centre of a token occupying `size` cells from `cell`.
 * A 1x1 token's centre is its cell's centre; a 2x2's is the corner where
 * its four cells meet. This is where the line is drawn from and to,
 * because a line between two top-left corners reads as off by half a
 * token on anything bigger than one square.
 */
export function tokenCentre(cell: Cell, size: TokenSize, gridSize: number): DrawingPoint {
	if (size.width === 1 && size.height === 1) return cellCentre(cell, gridSize);
	return { x: (cell.x + size.width / 2) * gridSize, y: (cell.y + size.height / 2) * gridSize };
}

/**
 * Everything the drag overlay needs: where the ghost sits, where the
 * line ends, and what the label says.
 *
 * The distance is counted corner cell to corner cell rather than centre
 * to centre, and that is right for a token of any size — `feetBetween`
 * works on the difference between two cells, and a token's two corners
 * are displaced by exactly as much as its two centres.
 */
export function tokenDragPreview(
	origin: Cell,
	size: TokenSize,
	position: DrawingPoint,
	gridSize: number
): TokenDragPreview {
	const cell = snapTokenCell(position, gridSize);
	const to = tokenCentre(cell, size, gridSize);
	return {
		cell,
		from: tokenCentre(origin, size, gridSize),
		to,
		labelAt: { x: to.x, y: cell.y * gridSize },
		label: `${feetBetween(origin, cell)} ft`,
		moved: cell.x !== origin.x || cell.y !== origin.y
	};
}
