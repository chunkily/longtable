// Geometric hit-testing for map drawings: "which stroke is under the
// pointer?", answered by measuring distance in world space rather than
// by sampling Konva's hit canvas.
//
// Konva's pixel-based hit graph can't do this reliably. It only accepts
// a pixel whose alpha is exactly 255, and canvas antialiasing means a
// thin stroke is mostly partial-coverage pixels, so it falls back to a
// spiral search of neighbouring pixels that can land on the wrong shape
// or find nothing at all. Its hit canvas also ignores devicePixelRatio,
// and any tolerance expressed as a stroke width scales with zoom.
// Measuring distance directly avoids all three: the tolerance is an
// explicit number the caller controls, and the answer doesn't depend on
// zoom level, display density, or how thin the stroke is drawn.

import type { Drawing, DrawingPoint } from './room.svelte';

/** Rendered width of a drawing's stroke, in world pixels. */
export const DRAWING_STROKE_WIDTH = 3;

// Drawings are all the same width and unfilled for now. When per-stroke
// widths and filled shapes land, these two read the values off the
// Drawing and nothing else in this module changes.
export function strokeWidthOf(drawing: Drawing): number {
	void drawing;
	return DRAWING_STROKE_WIDTH;
}

export function isFilled(drawing: Drawing): boolean {
	void drawing;
	return false;
}

/** Distance from p to the segment ab (to a, if the segment is a point). */
export function pointSegmentDistance(p: DrawingPoint, a: DrawingPoint, b: DrawingPoint): number {
	const dx = b.x - a.x;
	const dy = b.y - a.y;
	const lengthSquared = dx * dx + dy * dy;
	if (lengthSquared === 0) return Math.hypot(p.x - a.x, p.y - a.y);

	// How far along ab the perpendicular from p lands, clamped to the
	// segment so past-the-end measures to the endpoint.
	const t = Math.max(0, Math.min(1, ((p.x - a.x) * dx + (p.y - a.y) * dy) / lengthSquared));
	return Math.hypot(p.x - (a.x + t * dx), p.y - (a.y + t * dy));
}

function polylineDistance(points: DrawingPoint[], p: DrawingPoint): number {
	if (points.length === 0) return Infinity;
	if (points.length === 1) return Math.hypot(p.x - points[0].x, p.y - points[0].y);

	let nearest = Infinity;
	for (let i = 1; i < points.length; i++) {
		nearest = Math.min(nearest, pointSegmentDistance(p, points[i - 1], points[i]));
	}
	return nearest;
}

function rectBounds(a: DrawingPoint, b: DrawingPoint) {
	return {
		left: Math.min(a.x, b.x),
		right: Math.max(a.x, b.x),
		top: Math.min(a.y, b.y),
		bottom: Math.max(a.y, b.y)
	};
}

/** Distance to a rectangle's border, ignoring whether p is inside it. */
export function rectOutlineDistance(a: DrawingPoint, b: DrawingPoint, p: DrawingPoint): number {
	const { left, right, top, bottom } = rectBounds(a, b);
	const corners: DrawingPoint[] = [
		{ x: left, y: top },
		{ x: right, y: top },
		{ x: right, y: bottom },
		{ x: left, y: bottom }
	];
	return polylineDistance([...corners, corners[0]], p);
}

export function isInsideRect(a: DrawingPoint, b: DrawingPoint, p: DrawingPoint): boolean {
	const { left, right, top, bottom } = rectBounds(a, b);
	return p.x >= left && p.x <= right && p.y >= top && p.y <= bottom;
}

// An ellipse, like a rect, is stored as two opposite corners of the box
// it's inscribed in.
function ellipseBounds(a: DrawingPoint, b: DrawingPoint) {
	return {
		centreX: (a.x + b.x) / 2,
		centreY: (a.y + b.y) / 2,
		radiusX: Math.abs(b.x - a.x) / 2,
		radiusY: Math.abs(b.y - a.y) / 2
	};
}

// Exact point-to-ellipse distance means solving a quartic, and the usual
// shortcut — scaling the ellipse to a circle and measuring there —
// distorts distance along the major axis, which would make the eraser's
// reach vary around the perimeter. Walking a polyline around it instead
// reuses the segment distance used everywhere else here, and is accurate
// to within ELLIPSE_TOLERANCE of the true curve.
export const ELLIPSE_TOLERANCE = 0.1;
const MIN_ELLIPSE_SEGMENTS = 32;
const MAX_ELLIPSE_SEGMENTS = 512;

// Each chord sits about r·(π/n)²/2 inside the arc it spans, so holding
// that gap under a fixed tolerance means growing the segment count with
// the square root of the radius — a small ellipse needs far fewer
// segments than a battlefield-sized one to look the same to a click.
function ellipseSegments(radiusX: number, radiusY: number): number {
	const radius = Math.max(radiusX, radiusY);
	const needed = Math.ceil(Math.PI * Math.sqrt(radius / (2 * ELLIPSE_TOLERANCE)));
	return Math.min(MAX_ELLIPSE_SEGMENTS, Math.max(MIN_ELLIPSE_SEGMENTS, needed));
}

export function ellipseOutlineDistance(a: DrawingPoint, b: DrawingPoint, p: DrawingPoint): number {
	const { centreX, centreY, radiusX, radiusY } = ellipseBounds(a, b);
	const segments = ellipseSegments(radiusX, radiusY);

	const perimeter: DrawingPoint[] = [];
	for (let i = 0; i <= segments; i++) {
		const angle = (i / segments) * 2 * Math.PI;
		perimeter.push({
			x: centreX + radiusX * Math.cos(angle),
			y: centreY + radiusY * Math.sin(angle)
		});
	}
	return polylineDistance(perimeter, p);
}

export function isInsideEllipse(a: DrawingPoint, b: DrawingPoint, p: DrawingPoint): boolean {
	const { centreX, centreY, radiusX, radiusY } = ellipseBounds(a, b);
	if (radiusX === 0 || radiusY === 0) return false;

	const dx = (p.x - centreX) / radiusX;
	const dy = (p.y - centreY) / radiusY;
	return dx * dx + dy * dy <= 1;
}

/**
 * Distance from p to the drawing's centreline, in world units.
 *
 * An unfilled shape is only its outline — the empty space inside a
 * rectangle isn't part of it, so clicking through the middle of one
 * reaches whatever is behind it. A filled shape includes its interior,
 * and measures 0 anywhere inside, so erasing the inside erases the
 * whole shape.
 *
 * Returns Infinity for a drawing without enough points to have a shape.
 */
export function distanceToDrawing(drawing: Drawing, p: DrawingPoint): number {
	const points = drawing.points;
	if (drawing.kind === 'freehand') return polylineDistance(points, p);
	if (points.length < 2) return Infinity;

	const [a, b] = points;
	switch (drawing.kind) {
		case 'line':
			return pointSegmentDistance(p, a, b);
		case 'rect':
			if (isFilled(drawing) && isInsideRect(a, b, p)) return 0;
			return rectOutlineDistance(a, b, p);
		case 'ellipse':
			if (isFilled(drawing) && isInsideEllipse(a, b, p)) return 0;
			return ellipseOutlineDistance(a, b, p);
	}
}

/**
 * The drawing under point, or null. pickRadius is the slack around the
 * pointer, in world units — measured to the *edge* of a stroke's body,
 * so a fat stroke is hit anywhere on it and the rule reads as "the pick
 * circle touches the stroke". Later drawings render on top, so they win
 * both ties and overlaps.
 */
export function pickDrawing(
	drawings: Drawing[],
	point: DrawingPoint,
	pickRadius: number
): Drawing | null {
	let picked: Drawing | null = null;
	let nearest = Infinity;

	for (let i = drawings.length - 1; i >= 0; i--) {
		const drawing = drawings[i];
		const distance = distanceToDrawing(drawing, point) - strokeWidthOf(drawing) / 2;
		if (distance <= pickRadius && distance < nearest) {
			picked = drawing;
			nearest = distance;
		}
	}
	return picked;
}
