import { describe, expect, it } from 'vitest';
import {
	circleRadius,
	conePolygon,
	cubePolygon,
	cubeSide,
	linePolygon,
	quantiseFeet,
	quantiseTemplateEnd,
	snapPoint,
	templateFeet,
	templateLabel,
	templatePolygon
} from './aoe';
import type { DrawingPoint } from './room.svelte';

const GRID = 70;
const ORIGIN = { x: 0, y: 0 };

function sideLengths(polygon: DrawingPoint[]): number[] {
	return polygon.map((p, i) => {
		const q = polygon[(i + 1) % polygon.length];
		return Math.hypot(q.x - p.x, q.y - p.y);
	});
}

describe('snapPoint', () => {
	// The three modes exist because tables genuinely disagree about where
	// a template may sit, so each has to land somewhere different for the
	// same pointer position.
	it('leaves a point alone in free mode', () => {
		expect(snapPoint({ x: 33, y: 47 }, GRID, 'free')).toEqual({ x: 33, y: 47 });
	});

	it('takes a point to the nearest grid corner in intersections mode', () => {
		expect(snapPoint({ x: 10, y: 10 }, GRID, 'intersections')).toEqual({ x: 0, y: 0 });
		expect(snapPoint({ x: 60, y: 10 }, GRID, 'intersections')).toEqual({ x: 70, y: 0 });
		// Negative coordinates are ordinary: the map plane extends past the
		// scene's bounds in every direction. -0 would be a different origin
		// for a template dragged left of centre than one dragged right.
		expect(snapPoint({ x: -60, y: -10 }, GRID, 'intersections')).toEqual({ x: -70, y: 0 });
	});

	it('takes a point to the middle of the square it is in, in centres mode', () => {
		expect(snapPoint({ x: 10, y: 10 }, GRID, 'centres')).toEqual({ x: 35, y: 35 });
		expect(snapPoint({ x: 69, y: 1 }, GRID, 'centres')).toEqual({ x: 35, y: 35 });
		expect(snapPoint({ x: 71, y: 1 }, GRID, 'centres')).toEqual({ x: 105, y: 35 });
	});

	it('is a no-op without a usable grid', () => {
		expect(snapPoint({ x: 33, y: 47 }, 0, 'intersections')).toEqual({ x: 33, y: 47 });
	});
});

describe('sizes and labels', () => {
	it('measures a circle by its radius and a cone by its length', () => {
		expect(templateFeet('circle', ORIGIN, { x: 4 * GRID, y: 0 }, GRID)).toBe(20);
		expect(templateFeet('cone', ORIGIN, { x: 3 * GRID, y: 0 }, GRID)).toBe(15);
	});

	// A cube is named by its side, but dragged by its diagonal — so an
	// axis-aligned drag of 2 squares is a cube standing on a corner whose
	// side is 2/√2 squares, not a 10 ft one. That works out at 7.07 ft,
	// which is where the sizes no spell has used to come from; it now
	// reports the nearest real one.
	it('measures a cube by its side, not the diagonal that was dragged', () => {
		expect(cubeSide(ORIGIN, { x: 2 * GRID, y: 2 * GRID })).toBeCloseTo(2 * GRID);
		expect(templateFeet('cube', ORIGIN, { x: 2 * GRID, y: 2 * GRID }, GRID)).toBe(10);
		expect(templateFeet('cube', ORIGIN, { x: 2 * GRID, y: 0 }, GRID)).toBe(5);
	});

	it('names the shape, since the same number reads differently per shape', () => {
		const to = { x: 3 * GRID, y: 0 };
		expect(templateLabel('circle', ORIGIN, to, GRID)).toBe('15 ft radius');
		expect(templateLabel('cone', ORIGIN, to, GRID)).toBe('15 ft cone');
		expect(templateLabel('line', ORIGIN, to, GRID, 10)).toBe('15 ft line, 10 ft wide');
		expect(templateLabel('cube', ORIGIN, { x: 2 * GRID, y: 2 * GRID }, GRID)).toBe('10 ft cube');
	});
});

describe('quantising to whole area sizes', () => {
	it('rounds to the nearest 5 ft, which is every size the rules use', () => {
		expect(quantiseFeet(7)).toBe(5);
		expect(quantiseFeet(8)).toBe(10);
		expect(quantiseFeet(20)).toBe(20);
		expect(quantiseFeet(22.4)).toBe(20);
	});

	// Rounding to nearest alone sends anything under half a step to zero,
	// which would make a template vanish for the first few pixels of every
	// drag. A drag of nothing is still nothing, though — that is what
	// keeps a shape off the map on mousedown.
	it('never rounds a real drag away to nothing, but leaves a still pointer alone', () => {
		expect(quantiseFeet(1)).toBe(5);
		expect(quantiseFeet(2.4)).toBe(5);
		expect(quantiseFeet(0)).toBe(0);
	});

	// The case that started this: two grid corners one square apart
	// diagonally are 5·√2 ≈ 7.07 ft, so snapping the origin was never
	// going to be enough on its own.
	it('takes the diagonal between two snapped corners to a real size', () => {
		const end = quantiseTemplateEnd('circle', ORIGIN, { x: GRID, y: GRID }, GRID);

		expect(templateFeet('circle', ORIGIN, end, GRID)).toBe(5);
		expect(circleRadius(ORIGIN, end)).toBeCloseTo(GRID);
	});

	it('keeps the direction the drag gave it, and only changes the length', () => {
		const to = { x: 123, y: 47 };
		const end = quantiseTemplateEnd('circle', ORIGIN, to, GRID);

		// Same bearing to well past the precision anyone could point at.
		expect(Math.atan2(end.y, end.x)).toBeCloseTo(Math.atan2(to.y, to.x), 10);
		// 131.7 world units is 9.4 ft, which rounds up to 10.
		expect(templateFeet('circle', ORIGIN, end, GRID)).toBe(10);
	});

	// A cube is dragged along its diagonal and named by its side, so the
	// quantised side has to be stretched back out before it can say where
	// the pointer's corner belongs.
	it('quantises a cube by its side and returns the matching diagonal', () => {
		const end = quantiseTemplateEnd('cube', ORIGIN, { x: 2 * GRID, y: 0 }, GRID);

		expect(templateFeet('cube', ORIGIN, end, GRID)).toBe(5);
		expect(cubeSide(ORIGIN, end)).toBeCloseTo(GRID);
		expect(sideLengths(cubePolygon(ORIGIN, end)).every((s) => Math.abs(s - GRID) < 1e-9)).toBe(
			true
		);
	});

	it('leaves a drag that has not moved, and a scene with no grid, alone', () => {
		expect(quantiseTemplateEnd('circle', ORIGIN, ORIGIN, GRID)).toEqual(ORIGIN);
		expect(quantiseTemplateEnd('circle', ORIGIN, { x: 33, y: 47 }, 0)).toEqual({ x: 33, y: 47 });
	});
});

describe('cone', () => {
	// The PHB's rule, and the only thing that fixes the flare: a cone's
	// width at any point equals that point's distance from the origin.
	it('is exactly as wide at its base as it is long', () => {
		const length = 4 * GRID;
		const [apex, left, right] = conePolygon(ORIGIN, { x: length, y: 0 });

		expect(apex).toEqual(ORIGIN);
		expect(Math.hypot(left.x - right.x, left.y - right.y)).toBeCloseTo(length);
		expect(left.x).toBeCloseTo(length);
		expect(right.x).toBeCloseTo(length);
	});

	it('keeps that ratio whichever way it points', () => {
		for (const to of [
			{ x: 0, y: 5 * GRID },
			{ x: -3 * GRID, y: 0 },
			{ x: 2 * GRID, y: 2 * GRID }
		]) {
			const length = Math.hypot(to.x, to.y);
			const [, left, right] = conePolygon(ORIGIN, to);
			expect(Math.hypot(left.x - right.x, left.y - right.y)).toBeCloseTo(length);
		}
	});

	it('has no shape before the drag has moved', () => {
		expect(conePolygon(ORIGIN, ORIGIN)).toEqual([]);
	});
});

describe('line', () => {
	it('is a rectangle of the dragged length and the given width', () => {
		const polygon = linePolygon(ORIGIN, { x: 6 * GRID, y: 0 }, GRID);
		const sides = sideLengths(polygon);

		expect(polygon).toHaveLength(4);
		// Opposite sides pair up: two of the length, two of the width.
		expect(sides[0]).toBeCloseTo(6 * GRID);
		expect(sides[2]).toBeCloseTo(6 * GRID);
		expect(sides[1]).toBeCloseTo(GRID);
		expect(sides[3]).toBeCloseTo(GRID);
	});

	// Straddling the axis rather than sitting to one side means growing a
	// line doesn't slide it sideways under the pointer.
	it('is centred on the drag, not offset to one side of it', () => {
		const polygon = linePolygon(ORIGIN, { x: 4 * GRID, y: 0 }, GRID);
		const meanY = polygon.reduce((total, p) => total + p.y, 0) / polygon.length;
		expect(meanY).toBeCloseTo(0);
	});

	it('has no shape without a length or a width', () => {
		expect(linePolygon(ORIGIN, ORIGIN, GRID)).toEqual([]);
		expect(linePolygon(ORIGIN, { x: GRID, y: 0 }, 0)).toEqual([]);
	});
});

describe('cube', () => {
	// Two opposite corners fix a square completely, which is what lets one
	// drag set size and rotation together.
	it('closes into a square from the two dragged corners', () => {
		const polygon = cubePolygon(ORIGIN, { x: 2 * GRID, y: 2 * GRID });
		const sides = sideLengths(polygon);

		expect(polygon).toHaveLength(4);
		for (const side of sides) expect(side).toBeCloseTo(2 * GRID);
		// The dragged corners stay put, opposite each other.
		expect(polygon[0]).toEqual(ORIGIN);
		expect(polygon[2]).toEqual({ x: 2 * GRID, y: 2 * GRID });
	});

	it('stands on a corner as a diamond when dragged along an axis', () => {
		const polygon = cubePolygon(ORIGIN, { x: 2 * GRID, y: 0 });
		const sides = sideLengths(polygon);

		for (const side of sides) expect(side).toBeCloseTo(Math.SQRT2 * GRID);
		// The off-diagonal corners sit above and below the drag, which is
		// what makes it read as a diamond. Asserted as a pair rather than
		// in order: nothing downstream depends on the winding direction,
		// so pinning it here would only invent a constraint to break.
		expect(new Set([polygon[1], polygon[3]].map((p) => `${p.x},${p.y}`))).toEqual(
			new Set([`${GRID},${GRID}`, `${GRID},${-GRID}`])
		);
	});

	it('has right angles at every corner', () => {
		const polygon = cubePolygon(ORIGIN, { x: 3 * GRID, y: GRID });
		for (let i = 0; i < 4; i++) {
			const prev = polygon[(i + 3) % 4];
			const here = polygon[i];
			const next = polygon[(i + 1) % 4];
			const dot = (prev.x - here.x) * (next.x - here.x) + (prev.y - here.y) * (next.y - here.y);
			expect(dot).toBeCloseTo(0);
		}
	});

	it('has no shape before the drag has moved', () => {
		expect(cubePolygon(ORIGIN, ORIGIN)).toEqual([]);
	});
});

describe('templatePolygon', () => {
	it('routes each kind to its own shape', () => {
		const to = { x: 3 * GRID, y: 0 };
		expect(templatePolygon('cone', ORIGIN, to, GRID)).toHaveLength(3);
		expect(templatePolygon('line', ORIGIN, to, GRID)).toHaveLength(4);
		expect(templatePolygon('cube', ORIGIN, to, GRID)).toHaveLength(4);
		// A circle isn't a polygon; the canvas asks for its radius instead.
		expect(templatePolygon('circle', ORIGIN, to, GRID)).toEqual([]);
		expect(circleRadius(ORIGIN, to)).toBeCloseTo(3 * GRID);
	});
});
