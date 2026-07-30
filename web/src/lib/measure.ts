// Grid distance for the measuring tool. Kept out of the canvas
// component so the rule itself can be tested without a Konva stage —
// it's the part that has to be right, and the part nobody can eyeball
// from a screenshot.

import type { DrawingPoint } from './room.svelte';

/**
 * Feet one grid square represents. D&D's standard 5ft; fixed rather
 * than per-scene, since nothing in the scene model records a scale yet.
 */
export const FEET_PER_SQUARE = 5;

/**
 * Minimum gap between measurement updates put on the wire. A drag fires
 * pointer events far faster than anyone can read a changing number, and
 * the local line is drawn from local state regardless — this only paces
 * what other people's maps are told, and a trailing send makes sure the
 * position they end on is the one the drag ended on.
 */
export const MEASURE_SEND_INTERVAL_MS = 40;

export interface Cell {
	x: number;
	y: number;
}

/** The grid cell containing a world-space point. */
export function cellAt(point: DrawingPoint, gridSize: number): Cell {
	return { x: Math.floor(point.x / gridSize), y: Math.floor(point.y / gridSize) };
}

/** The centre of a cell, in world space — where a measurement line is drawn to. */
export function cellCentre(cell: Cell, gridSize: number): DrawingPoint {
	return { x: (cell.x + 0.5) * gridSize, y: (cell.y + 0.5) * gridSize };
}

/**
 * Squares travelled between two cells under the alternating diagonal
 * rule: the 1st diagonal step of a move costs 1 square, the 2nd costs 2,
 * the 3rd 1 again, and so on. Every second diagonal therefore costs an
 * extra square on top of the straight-line square count.
 *
 * Not Euclidean distance: a knight's-move away is 3 squares here, not
 * 2.24 — the point is to answer "can I reach it", which is counted in
 * squares of movement.
 */
export function squaresBetween(from: Cell, to: Cell): number {
	const dx = Math.abs(to.x - from.x);
	const dy = Math.abs(to.y - from.y);
	const diagonals = Math.min(dx, dy);
	const straights = Math.max(dx, dy) - diagonals;
	return straights + diagonals + Math.floor(diagonals / 2);
}

/** squaresBetween, in feet. */
export function feetBetween(from: Cell, to: Cell): number {
	return squaresBetween(from, to) * FEET_PER_SQUARE;
}

/**
 * Distance between two world-space points, as the label shown on the
 * map: the cells they fall in, counted by the diagonal rule.
 */
export function measureLabel(from: DrawingPoint, to: DrawingPoint, gridSize: number): string {
	return `${feetBetween(cellAt(from, gridSize), cellAt(to, gridSize))} ft`;
}
