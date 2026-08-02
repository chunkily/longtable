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
//
// Sizes are quantised to 5 ft, which is a different question from the
// one above and answered the other way. Refusing to say which squares an
// area catches is refusing to adjudicate a disagreement; rounding a size
// to 5 ft is repeating something the rules already settled, since no
// printed area is anything else. A drag that produced a 7 ft cone wasn't
// offering a choice, it was offering a spell nobody has.

import { FEET_PER_SQUARE } from './measure';
import type { DrawingPoint } from './room.svelte';

/** The template shapes a Room Member can drag out. */
export type TemplateKind = 'circle' | 'cone' | 'line' | 'cube';

/**
 * Where a template's **point of origin** is allowed to land. Tables
 * genuinely disagree here — some place a burst on a cell centre, some on
 * an intersection, some eyeball it — so this is a setting rather than a
 * rule, and it only ever affects the person dragging: the points that go
 * on the wire are already snapped.
 *
 * It governs the origin alone. The far end of the drag isn't snapped,
 * because quantiseTemplateEnd moves it anyway — the drag's *direction*
 * is taken as given and its *length* is rounded to a whole area size.
 * Snapping it first would only coarsen the direction, which is the one
 * thing a drag is genuinely good at expressing.
 */
export type SnapMode = 'free' | 'centres' | 'intersections';

/**
 * The increment every area size is rounded to. Every area in the 2024
 * PHB is a multiple of 5 ft — there is no 7 ft cone — so a drag landing
 * between increments describes a spell nobody has.
 *
 * Kept separate from FEET_PER_SQUARE despite both being 5 today. That
 * one is how much ground a square covers; this one is the increment the
 * rules are written in. A scene that one day records its own scale
 * should change the first without touching the second.
 */
export const TEMPLATE_STEP_FEET = 5;

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
	return quantiseFeet(worldToFeet(length, gridSize));
}

/**
 * A size in feet rounded to the increment the rules use, and never less
 * than one step once a drag has started. The clamp matters: rounding to
 * nearest alone sends anything under half a step to zero, so a template
 * would wink out of existence for the first few pixels of every drag.
 * A drag of nothing at all is still nothing — that's what keeps a shape
 * from appearing on mousedown, before there's a direction to give it.
 */
export function quantiseFeet(feet: number): number {
	if (feet <= 0) return 0;
	return Math.max(TEMPLATE_STEP_FEET, Math.round(feet / TEMPLATE_STEP_FEET) * TEMPLATE_STEP_FEET);
}

/**
 * The drag's far end moved to the nearest whole area size, keeping the
 * direction the drag gave it.
 *
 * This is what stops a 7 ft cone existing. Snapping the origin doesn't
 * achieve it on its own — a one-square *diagonal* between two corners is
 * 5·√2 ≈ 7.07 ft, so the tidiest possible drag still produced a size no
 * spell has. Quantising the length is the only thing that fixes it, and
 * it has to move the point rather than only the label, or the outline
 * goes on drawing a size it isn't claiming to be.
 *
 * Applied on the client before anything goes on the wire, exactly as
 * snapping is: what other people receive is already final.
 */
export function quantiseTemplateEnd(
	kind: TemplateKind,
	from: DrawingPoint,
	to: DrawingPoint,
	gridSize: number
): DrawingPoint {
	const dragged = distanceBetween(from, to);
	if (dragged === 0 || gridSize <= 0) return to;

	const feet = templateFeet(kind, from, to, gridSize);
	// A cube is dragged along its diagonal but named by its side, so the
	// quantised side has to be stretched back out to a diagonal before it
	// can say where the pointer's corner belongs.
	const side = feetToWorld(feet, gridSize);
	const length = kind === 'cube' ? side * Math.SQRT2 : side;

	return {
		x: from.x + ((to.x - from.x) / dragged) * length,
		y: from.y + ((to.y - from.y) / dragged) * length
	};
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
