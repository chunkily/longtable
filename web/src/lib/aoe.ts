// Area-of-effect templates: the shapes themselves, in world space.
// Kept out of the canvas component for the same reason as $lib/measure —
// it's the part that has to be right, and the part nobody can eyeball
// from a screenshot.
//
// **These compute the shape, never the squares it catches.** That's a
// deliberate refusal, not an omission. Tables disagree about whether a
// sphere sits on a cell centre or an intersection, and about whether a
// 5 ft cube covers one square or four; a highlight would be this app
// picking a side, and picking it invisibly. Drawing the true shape and
// leaving the reading to the players is what a paper cutout laid on a
// battle map always did, and it's the only version that's right for
// every table.
//
// The 2024 PHB names six area shapes, which come down to four outlines
// once flattened onto a top-down map:
//
//   Sphere, Cylinder, Emanation -> a circle. A Cylinder's height is off
//     the plane, and an Emanation differs only in being anchored to a
//     creature — which matters when something persists and has to
//     follow a token, not while a template is being dragged out.
//   Cone   -> a triangle. "A Cone's width at any point along its length
//     is equal to that point's distance from the point of origin", so
//     the flare is fixed at ~53 degrees rather than being a choice.
//   Line   -> a rectangle. The only shape whose size a single drag
//     can't express: length and direction come from the drag, width has
//     to be supplied.
//   Cube   -> a square, taken from two opposite corners so one drag
//     gives size and rotation together.

import { FEET_PER_SQUARE } from './measure';
import type { DrawingPoint } from './room.svelte';

/** The template shapes a Room Member can drag out. */
export type TemplateKind = 'circle' | 'cone' | 'line' | 'cube';

/**
 * Where a template's points are allowed to land. Tables genuinely
 * disagree here — some place a burst on a cell centre, some on an
 * intersection, some eyeball it — so this is a setting rather than a
 * rule, and it only ever affects the person dragging: the points that
 * go on the wire are already snapped.
 */
export type SnapMode = 'free' | 'centres' | 'intersections';

/** Default line width in feet, the narrowest a Line ever gets. */
export const DEFAULT_LINE_WIDTH_FEET = 5;

/** Widths offered for Line templates, covering the printed spells. */
export const LINE_WIDTH_CHOICES_FEET = [5, 10, 15, 20];

/** Applies a snap mode to a world-space point. */
export function snapPoint(point: DrawingPoint, gridSize: number, mode: SnapMode): DrawingPoint {
	if (gridSize <= 0 || mode === 'free') return point;
	if (mode === 'centres') {
		return {
			x: (Math.floor(point.x / gridSize) + 0.5) * gridSize,
			y: (Math.floor(point.y / gridSize) + 0.5) * gridSize
		};
	}
	return {
		x: positiveZero(Math.round(point.x / gridSize) * gridSize),
		y: positiveZero(Math.round(point.y / gridSize) * gridSize)
	};
}

// Rounding a small negative coordinate gives -0, which is arithmetically
// equal to 0 but distinguishable by Object.is and by test equality — so a
// template dragged just left of the origin would report a different
// origin than one dragged just right of it, for no reason anyone can see.
function positiveZero(n: number): number {
	return n === 0 ? 0 : n;
}

/** World-space distance between two points. */
export function distanceBetween(a: DrawingPoint, b: DrawingPoint): number {
	return Math.hypot(b.x - a.x, b.y - a.y);
}

/** A world-space length in feet, to the nearest foot. */
export function worldToFeet(length: number, gridSize: number): number {
	if (gridSize <= 0) return 0;
	return Math.round((length / gridSize) * FEET_PER_SQUARE);
}

/** Feet back to world units — how a width in feet becomes a rectangle. */
export function feetToWorld(feet: number, gridSize: number): number {
	return (feet / FEET_PER_SQUARE) * gridSize;
}

/**
 * A cube's side, which is its diagonal over root two — the drag sets
 * two opposite corners, and the size a spell names is the side.
 */
export function cubeSide(from: DrawingPoint, to: DrawingPoint): number {
	return distanceBetween(from, to) / Math.SQRT2;
}

/** The size a spell would call this template, in feet. */
export function templateFeet(
	kind: TemplateKind,
	from: DrawingPoint,
	to: DrawingPoint,
	gridSize: number
): number {
	const length = kind === 'cube' ? cubeSide(from, to) : distanceBetween(from, to);
	return worldToFeet(length, gridSize);
}

/** The floating label: the size, and which shape is being measured. */
export function templateLabel(
	kind: TemplateKind,
	from: DrawingPoint,
	to: DrawingPoint,
	gridSize: number,
	lineWidthFeet = DEFAULT_LINE_WIDTH_FEET
): string {
	const feet = templateFeet(kind, from, to, gridSize);
	switch (kind) {
		case 'circle':
			return `${feet} ft radius`;
		case 'cone':
			return `${feet} ft cone`;
		case 'line':
			return `${feet} ft line, ${lineWidthFeet} ft wide`;
		case 'cube':
			return `${feet} ft cube`;
	}
}

/**
 * A template's outline as a closed polygon in world space, or an empty
 * array when the drag is too small to have a shape yet. Circles aren't
 * polygons and come back empty here — see circleRadius.
 */
export function templatePolygon(
	kind: TemplateKind,
	from: DrawingPoint,
	to: DrawingPoint,
	gridSize: number,
	lineWidthFeet = DEFAULT_LINE_WIDTH_FEET
): DrawingPoint[] {
	switch (kind) {
		case 'cone':
			return conePolygon(from, to);
		case 'line':
			return linePolygon(from, to, feetToWorld(lineWidthFeet, gridSize));
		case 'cube':
			return cubePolygon(from, to);
		case 'circle':
			return [];
	}
}

/** A circle template's radius in world units. */
export function circleRadius(from: DrawingPoint, to: DrawingPoint): number {
	return distanceBetween(from, to);
}

/**
 * The cone as a triangle: apex at the origin, and a base as wide as the
 * cone is long, which is the PHB's definition rather than a choice of
 * angle.
 */
export function conePolygon(from: DrawingPoint, to: DrawingPoint): DrawingPoint[] {
	const length = distanceBetween(from, to);
	if (length === 0) return [];

	const ux = (to.x - from.x) / length;
	const uy = (to.y - from.y) / length;
	const half = length / 2;

	return [
		from,
		{ x: to.x - uy * half, y: to.y + ux * half },
		{ x: to.x + uy * half, y: to.y - ux * half }
	];
}

/**
 * The line as a rectangle centred on the drag: the drag is the line's
 * axis and the width straddles it, so lengthening a line doesn't shift
 * which side of the axis it covers.
 */
export function linePolygon(from: DrawingPoint, to: DrawingPoint, width: number): DrawingPoint[] {
	const length = distanceBetween(from, to);
	if (length === 0 || width <= 0) return [];

	// Perpendicular to the axis, scaled to half the width.
	const px = (-(to.y - from.y) / length) * (width / 2);
	const py = ((to.x - from.x) / length) * (width / 2);

	return [
		{ x: from.x + px, y: from.y + py },
		{ x: to.x + px, y: to.y + py },
		{ x: to.x - px, y: to.y - py },
		{ x: from.x - px, y: from.y - py }
	];
}

/**
 * The cube from two opposite corners. Those two fix the square
 * completely: the other pair is the same diagonal rotated a quarter
 * turn about the centre, so one drag sets size *and* rotation — drag
 * along an axis and it stands on a corner as a diamond, drag diagonally
 * and it comes out square to the grid.
 */
export function cubePolygon(from: DrawingPoint, to: DrawingPoint): DrawingPoint[] {
	if (from.x === to.x && from.y === to.y) return [];

	const cx = (from.x + to.x) / 2;
	const cy = (from.y + to.y) / 2;
	// Half-diagonal, and the same vector turned 90 degrees.
	const hx = from.x - cx;
	const hy = from.y - cy;

	return [from, { x: cx - hy, y: cy + hx }, to, { x: cx + hy, y: cy - hx }];
}
