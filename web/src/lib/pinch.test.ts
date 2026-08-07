import { describe, expect, it } from 'vitest';
import { pinchStep, touchCentre, touchDistance, type PinchStep } from './pinch';

const BOUNDS = { minScale: 0.2, maxScale: 4 };

function step(overrides: Partial<PinchStep> = {}): PinchStep {
	return {
		scale: 1,
		position: { x: 0, y: 0 },
		from: { x: 100, y: 100 },
		to: { x: 100, y: 100 },
		ratio: 1,
		...BOUNDS,
		...overrides
	};
}

describe('touchDistance / touchCentre', () => {
	it('measures the gap between two touches and finds their midpoint', () => {
		const a = { x: 0, y: 0 };
		const b = { x: 30, y: 40 };
		expect(touchDistance(a, b)).toBe(50);
		expect(touchCentre(a, b)).toEqual({ x: 15, y: 20 });
	});

	it('is order-independent, since which finger landed first means nothing', () => {
		const a = { x: 12, y: -5 };
		const b = { x: -3, y: 9 };
		expect(touchDistance(a, b)).toBeCloseTo(touchDistance(b, a));
		expect(touchCentre(a, b)).toEqual(touchCentre(b, a));
	});
});

describe('pinchStep', () => {
	// The whole point of the gesture: whatever was under the fingers is
	// still under them afterwards, or the map swims away while you zoom.
	it('keeps the world point under the fingers fixed while scaling', () => {
		const before = step({ scale: 1, position: { x: 0, y: 0 }, ratio: 2 });
		const worldUnderCentre = {
			x: (before.from.x - before.position.x) / before.scale,
			y: (before.from.y - before.position.y) / before.scale
		};

		const after = pinchStep(before);

		expect(after.scale).toBe(2);
		// Project that same world point back to the screen with the new
		// transform; it should land where the fingers still are.
		expect(after.position.x + worldUnderCentre.x * after.scale).toBeCloseTo(before.to.x);
		expect(after.position.y + worldUnderCentre.y * after.scale).toBeCloseTo(before.to.y);
	});

	it('holds the anchor when the stage was already panned and zoomed', () => {
		const before = step({
			scale: 1.7,
			position: { x: -220, y: 90 },
			from: { x: 300, y: 210 },
			to: { x: 300, y: 210 },
			ratio: 1.25
		});
		const world = {
			x: (before.from.x - before.position.x) / before.scale,
			y: (before.from.y - before.position.y) / before.scale
		};

		const after = pinchStep(before);

		expect(after.position.x + world.x * after.scale).toBeCloseTo(before.to.x);
		expect(after.position.y + world.y * after.scale).toBeCloseTo(before.to.y);
	});

	// Two-finger panning isn't a separate feature — it's what anchoring on
	// the previous midpoint gives you, and it has to survive a ratio of 1.
	it('translates the map when both fingers move without spreading', () => {
		const after = pinchStep(
			step({
				position: { x: 10, y: 20 },
				from: { x: 100, y: 100 },
				to: { x: 130, y: 85 },
				ratio: 1
			})
		);

		expect(after.scale).toBe(1);
		expect(after.position).toEqual({ x: 40, y: 5 });
	});

	it('pans and zooms together when the fingers both move and spread', () => {
		const before = step({ from: { x: 100, y: 100 }, to: { x: 140, y: 100 }, ratio: 2 });
		const world = {
			x: (before.from.x - before.position.x) / before.scale,
			y: (before.from.y - before.position.y) / before.scale
		};

		const after = pinchStep(before);

		expect(after.scale).toBe(2);
		// The anchor follows the fingers rather than staying put.
		expect(after.position.x + world.x * after.scale).toBeCloseTo(140);
	});

	it('clamps to the same bounds the wheel respects, in both directions', () => {
		expect(pinchStep(step({ scale: 3, ratio: 10 })).scale).toBe(BOUNDS.maxScale);
		expect(pinchStep(step({ scale: 0.3, ratio: 0.01 })).scale).toBe(BOUNDS.minScale);
	});

	// Clamping must not leave the map sliding: once you're at the limit,
	// continuing to spread your fingers should do nothing at all.
	it('holds the anchor even when the scale is clamped', () => {
		const before = step({ scale: 4, position: { x: -50, y: -50 }, ratio: 3 });
		const world = {
			x: (before.from.x - before.position.x) / before.scale,
			y: (before.from.y - before.position.y) / before.scale
		};

		const after = pinchStep(before);

		expect(after.scale).toBe(BOUNDS.maxScale);
		expect(after.position.x + world.x * after.scale).toBeCloseTo(before.to.x);
		expect(after.position.y + world.y * after.scale).toBeCloseTo(before.to.y);
	});

	// A zero previous distance means both touches were on the same pixel,
	// which a fast gesture produces on its first move. Throwing out of a
	// touch handler would strand the gesture; not scaling is the honest
	// answer for one frame.
	it('treats an unusable ratio as no zoom rather than throwing', () => {
		for (const ratio of [Number.POSITIVE_INFINITY, Number.NaN, 0, -2]) {
			const after = pinchStep(step({ ratio, to: { x: 120, y: 100 } }));
			expect(after.scale).toBe(1);
			expect(after.position).toEqual({ x: 20, y: 0 });
		}
	});

	it('leaves the stage exactly where it was when nothing moved', () => {
		const after = pinchStep(step({ scale: 2.5, position: { x: 33, y: -14 } }));
		expect(after.scale).toBe(2.5);
		expect(after.position.x).toBeCloseTo(33);
		expect(after.position.y).toBeCloseTo(-14);
	});
});
