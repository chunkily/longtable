import { describe, expect, it } from 'vitest';
import {
	DRAWING_STROKE_WIDTH,
	ELLIPSE_TOLERANCE,
	distanceToDrawing,
	ellipseOutlineDistance,
	isInsideEllipse,
	isInsideRect,
	pickDrawing,
	pointSegmentDistance,
	rectOutlineDistance
} from './drawing-hit';
import type { Drawing, DrawingKind, DrawingPoint } from './room.svelte';

function drawing(kind: DrawingKind, points: DrawingPoint[], id: string = kind): Drawing {
	return { id, sceneId: 's1', kind, points, color: '#000000', createdByParticipantId: 'p1' };
}

// Ellipse distance is a polyline approximation of the true curve, so
// these assert the tolerance the module promises rather than an exact
// value. Anything looser would stop catching a genuinely wrong answer.
function expectNearCurve(actual: number, expected: number) {
	expect(Math.abs(actual - expected)).toBeLessThanOrEqual(ELLIPSE_TOLERANCE);
}

describe('pointSegmentDistance', () => {
	it('measures perpendicular distance to the segment', () => {
		expect(pointSegmentDistance({ x: 5, y: 3 }, { x: 0, y: 0 }, { x: 10, y: 0 })).toBe(3);
	});

	it('measures to the nearer endpoint past either end', () => {
		expect(pointSegmentDistance({ x: -4, y: 0 }, { x: 0, y: 0 }, { x: 10, y: 0 })).toBe(4);
		expect(pointSegmentDistance({ x: 13, y: 4 }, { x: 0, y: 0 }, { x: 10, y: 0 })).toBe(5);
	});

	it('handles a zero-length segment', () => {
		expect(pointSegmentDistance({ x: 3, y: 4 }, { x: 0, y: 0 }, { x: 0, y: 0 })).toBe(5);
	});
});

describe('distanceToDrawing', () => {
	it('measures a line from its centreline', () => {
		const d = drawing('line', [
			{ x: 0, y: 0 },
			{ x: 100, y: 0 }
		]);
		expect(distanceToDrawing(d, { x: 50, y: 0 })).toBe(0);
		expect(distanceToDrawing(d, { x: 50, y: 9 })).toBe(9);
	});

	it('measures freehand from its nearest segment', () => {
		const d = drawing('freehand', [
			{ x: 0, y: 0 },
			{ x: 50, y: 0 },
			{ x: 50, y: 50 }
		]);
		expect(distanceToDrawing(d, { x: 25, y: 4 })).toBe(4);
		expect(distanceToDrawing(d, { x: 54, y: 25 })).toBe(4);
	});

	it('measures a rect from its border, not its area', () => {
		const d = drawing('rect', [
			{ x: 0, y: 0 },
			{ x: 100, y: 100 }
		]);
		expect(distanceToDrawing(d, { x: 0, y: 50 })).toBe(0);
		expect(distanceToDrawing(d, { x: 3, y: 50 })).toBe(3);
		// dead centre of an unfilled rect is 50 from every edge, so it is
		// not "on" the shape at all
		expect(distanceToDrawing(d, { x: 50, y: 50 })).toBe(50);
	});

	// An ellipse is inscribed in the box the drag defines, so these
	// corners describe a circle of radius 50 centred at (50, 50).
	it('measures an ellipse from its perimeter, not its centre', () => {
		const d = drawing('ellipse', [
			{ x: 0, y: 0 },
			{ x: 100, y: 100 }
		]);
		expectNearCurve(distanceToDrawing(d, { x: 100, y: 50 }), 0);
		expectNearCurve(distanceToDrawing(d, { x: 104, y: 50 }), 4);
		expectNearCurve(distanceToDrawing(d, { x: 96, y: 50 }), 4);
		expectNearCurve(distanceToDrawing(d, { x: 50, y: 50 }), 50);
	});

	it('measures a squashed ellipse correctly on both axes', () => {
		// centred at (0, 0), 200 wide and 40 tall
		const d = drawing('ellipse', [
			{ x: -100, y: -20 },
			{ x: 100, y: 20 }
		]);
		expectNearCurve(distanceToDrawing(d, { x: 105, y: 0 }), 5);
		expectNearCurve(distanceToDrawing(d, { x: 0, y: 25 }), 5);
		// Inside the bounding box but well clear of the curve, up near a
		// corner the old centre-and-radius circle would have covered.
		expect(distanceToDrawing(d, { x: 95, y: 18 })).toBeGreaterThan(5);
	});

	// A big ellipse must not be sloppier than a small one: the segment
	// count grows with the radius to hold the same tolerance.
	it('holds its tolerance at battlefield scale', () => {
		const d = drawing('ellipse', [
			{ x: 0, y: 0 },
			{ x: 4000, y: 4000 }
		]);
		expectNearCurve(distanceToDrawing(d, { x: 2000, y: 2000 }), 2000);
		expectNearCurve(distanceToDrawing(d, { x: 4010, y: 2000 }), 10);
	});

	it('is corner-order agnostic for ellipses', () => {
		const p = { x: 30, y: 5 };
		const forwards = drawing('ellipse', [
			{ x: 0, y: 0 },
			{ x: 60, y: 40 }
		]);
		const backwards = drawing('ellipse', [
			{ x: 60, y: 40 },
			{ x: 0, y: 0 }
		]);
		expect(distanceToDrawing(forwards, p)).toBeCloseTo(distanceToDrawing(backwards, p), 6);
	});

	it('is corner-order agnostic for rects', () => {
		const topLeftFirst = drawing('rect', [
			{ x: 0, y: 0 },
			{ x: 100, y: 60 }
		]);
		const bottomRightFirst = drawing('rect', [
			{ x: 100, y: 60 },
			{ x: 0, y: 0 }
		]);
		const p = { x: 20, y: 64 };
		expect(distanceToDrawing(topLeftFirst, p)).toBe(distanceToDrawing(bottomRightFirst, p));
	});

	it('ignores drawings with too few points to have a shape', () => {
		expect(distanceToDrawing(drawing('line', []), { x: 0, y: 0 })).toBe(Infinity);
		expect(distanceToDrawing(drawing('freehand', []), { x: 0, y: 0 })).toBe(Infinity);
	});
});

// Filled shapes aren't drawn yet, but when they are, clicking anywhere
// inside one should erase the whole shape — so the interior tests the
// geometry needs are here and correct ahead of that.
describe('interior tests for filled shapes', () => {
	const a = { x: 0, y: 0 };
	const b = { x: 100, y: 50 };

	it('recognises a rect interior, border, and exterior', () => {
		expect(isInsideRect(a, b, { x: 50, y: 25 })).toBe(true);
		expect(isInsideRect(a, b, { x: 0, y: 25 })).toBe(true);
		expect(isInsideRect(a, b, { x: -1, y: 25 })).toBe(false);
		expect(isInsideRect(a, b, { x: 50, y: 51 })).toBe(false);
	});

	it('measures rect outline distance independently of being inside', () => {
		expect(rectOutlineDistance(a, b, { x: 50, y: 25 })).toBe(25);
		expect(rectOutlineDistance(a, b, { x: 50, y: 55 })).toBe(5);
	});

	it('recognises an ellipse interior, and excludes the box corners', () => {
		expect(isInsideEllipse(a, b, { x: 50, y: 25 })).toBe(true);
		expect(isInsideEllipse(a, b, { x: 100, y: 25 })).toBe(true);
		// inside the bounding box, outside the curve
		expect(isInsideEllipse(a, b, { x: 0, y: 0 })).toBe(false);
		expect(isInsideEllipse(a, b, { x: 100, y: 50 })).toBe(false);
	});

	it('treats a flat ellipse as having no interior', () => {
		expect(isInsideEllipse({ x: 0, y: 0 }, { x: 100, y: 0 }, { x: 50, y: 0 })).toBe(false);
	});

	it('measures ellipse outline distance independently of being inside', () => {
		expectNearCurve(ellipseOutlineDistance(a, b, { x: 50, y: 25 }), 25);
		expectNearCurve(ellipseOutlineDistance(a, b, { x: 50, y: 55 }), 5);
	});
});

describe('pickDrawing', () => {
	const line = drawing('line', [
		{ x: 0, y: 0 },
		{ x: 100, y: 0 }
	]);

	it('picks a stroke within reach and ignores one outside it', () => {
		expect(pickDrawing([line], { x: 50, y: 10 }, 12)?.id).toBe('line');
		expect(pickDrawing([line], { x: 50, y: 40 }, 12)).toBeNull();
	});

	it('measures to the edge of the stroke, not its centreline', () => {
		// reach of 1 plus half the stroke's own width still connects
		const justInside = 1 + DRAWING_STROKE_WIDTH / 2;
		expect(pickDrawing([line], { x: 50, y: justInside }, 1)?.id).toBe('line');
		expect(pickDrawing([line], { x: 50, y: justInside + 0.5 }, 1)).toBeNull();
	});

	it('gives the same reach at any zoom, given a world radius for it', () => {
		// The caller converts a screen-pixel reach to world units; at 4x
		// zoom 12 screen px is 3 world px, at 0.25x it is 48.
		expect(pickDrawing([line], { x: 50, y: 4 }, 12 / 4)?.id).toBe('line');
		expect(pickDrawing([line], { x: 50, y: 40 }, 12 / 0.25)?.id).toBe('line');
	});

	it('prefers the nearest of several candidates', () => {
		const far = drawing(
			'line',
			[
				{ x: 0, y: 0 },
				{ x: 100, y: 0 }
			],
			'far'
		);
		const near = drawing(
			'line',
			[
				{ x: 0, y: 20 },
				{ x: 100, y: 20 }
			],
			'near'
		);
		expect(pickDrawing([far, near], { x: 50, y: 18 }, 12)?.id).toBe('near');
	});

	it('breaks a tie in favour of the one drawn last, which renders on top', () => {
		const under = drawing(
			'line',
			[
				{ x: 0, y: 0 },
				{ x: 100, y: 0 }
			],
			'under'
		);
		const over = drawing(
			'line',
			[
				{ x: 0, y: 0 },
				{ x: 100, y: 0 }
			],
			'over'
		);
		expect(pickDrawing([under, over], { x: 50, y: 2 }, 12)?.id).toBe('over');
	});

	it('returns null when there is nothing to pick', () => {
		expect(pickDrawing([], { x: 0, y: 0 }, 12)).toBeNull();
	});
});
