import { describe, expect, it } from 'vitest';
import { isPanButton, panStep, type Point } from './pan';

describe('isPanButton', () => {
	it('takes the right and middle buttons and leaves the left alone', () => {
		expect(isPanButton(2)).toBe(true);
		expect(isPanButton(1)).toBe(true);
		// The left button belongs to the active tool, and to Konva's own
		// stage drag when there isn't one. Claiming it here would give the
		// map two panning mechanisms fighting over the same gesture.
		expect(isPanButton(0)).toBe(false);
	});

	it('ignores the back and forward buttons a mouse may also have', () => {
		expect(isPanButton(3)).toBe(false);
		expect(isPanButton(4)).toBe(false);
	});

	// These are `MouseEvent.button` numbers, and the `buttons` bitmask
	// numbers the same physical buttons differently — 1 is left in the
	// mask and middle here. A caller reaching for one and passing the
	// other is the mistake this exists to make loud.
	it('is asked about a button number, in which 1 is the middle button', () => {
		expect(isPanButton(1)).toBe(true);
	});
});

describe('panStep', () => {
	it('moves the stage by exactly what the pointer moved', () => {
		const after = panStep({
			origin: { x: 0, y: 0 },
			from: { x: 100, y: 100 },
			to: { x: 140, y: 70 }
		});
		expect(after).toEqual({ x: 40, y: -30 });
	});

	it('adds that movement to wherever the stage already was', () => {
		const after = panStep({
			origin: { x: -220, y: 90 },
			from: { x: 300, y: 210 },
			to: { x: 260, y: 260 }
		});
		expect(after).toEqual({ x: -260, y: 140 });
	});

	// The reason this is anchored to the press rather than accumulated
	// between consecutive pointer samples. A real drag arrives as dozens
	// of small moves; a test drag as one jump. Anchoring makes those the
	// same answer, so neither the gesture's frame rate nor a dropped
	// sample can leave the map short of where the pointer is.
	it('lands in the same place however many samples the drag arrives in', () => {
		const origin: Point = { x: 12, y: -8 };
		const from: Point = { x: 400, y: 300 };
		const to: Point = { x: 190, y: 415 };

		const inOneJump = panStep({ origin, from, to });

		let stepped = origin;
		for (let i = 1; i <= 50; i++) {
			const t = i / 50;
			stepped = panStep({
				origin,
				from,
				to: { x: from.x + (to.x - from.x) * t, y: from.y + (to.y - from.y) * t }
			});
		}

		expect(stepped).toEqual(inOneJump);
	});

	// Scale is deliberately not a parameter: the stage's translation is
	// applied after its scale, so a pointer that moved 100px right wants
	// the translation 100px right whatever the zoom. A version that
	// divided by the scale looked right at 1x and crawled at 4x.
	it('is the same translation at every zoom level, because scale plays no part', () => {
		const args = { origin: { x: 5, y: 5 }, from: { x: 0, y: 0 }, to: { x: 100, y: 0 } };
		// Nothing to vary — that is the assertion. The screen-space delta
		// is the whole input, so there is no zoom-dependent branch to get
		// wrong.
		expect(panStep(args)).toEqual({ x: 105, y: 5 });
	});

	// The property the whole mid-gesture behaviour rests on, and the
	// reason a right-drag doesn't disturb a ruler being dragged out: the
	// stage moves by exactly what the pointer moved, so the world point
	// under the cursor is unchanged for the length of the pan. The tool's
	// own mousemove keeps firing throughout and keeps arriving at the same
	// answer, which is why nothing had to be written to protect it.
	it('holds the world point under the cursor still for the whole pan', () => {
		const scale = 2.5;
		const stage: Point = { x: -140, y: 60 };
		const press: Point = { x: 500, y: 400 };
		const worldAtPress = { x: (press.x - stage.x) / scale, y: (press.y - stage.y) / scale };

		for (const pointer of [
			{ x: 460, y: 380 },
			{ x: 390, y: 300 },
			{ x: 275, y: 190 }
		]) {
			const next = panStep({ origin: stage, from: press, to: pointer });
			expect({ x: (pointer.x - next.x) / scale, y: (pointer.y - next.y) / scale }).toEqual(
				worldAtPress
			);
		}
	});

	it('leaves the stage where it was when the pointer never moved', () => {
		const origin = { x: 33, y: -14 };
		expect(panStep({ origin, from: { x: 60, y: 60 }, to: { x: 60, y: 60 } })).toEqual(origin);
	});
});
