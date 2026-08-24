import { describe, expect, it } from 'vitest';
import { frameWidth, previewScale } from './grid-zoom';

describe('frameWidth', () => {
	it('hands fit mode to CSS rather than computing a width', () => {
		// A computed width here would be a feedback loop: the width out
		// changes the layout the width in was measured from.
		expect(frameWidth('fit', 2100)).toBe('100%');
	});

	it('asks for native scale at 1, so the art can be read pixel for pixel', () => {
		expect(frameWidth(1, 2100)).toBe('2100px');
	});

	it('multiplies past native scale', () => {
		expect(frameWidth(2, 2100)).toBe('4200px');
	});

	it('falls back to fit before the image has been measured', () => {
		// naturalWidth is 0 until the file loads, and "0px" would render a
		// staged map as nothing at all.
		expect(frameWidth(1, 0)).toBe('100%');
		expect(frameWidth(2, Number.NaN)).toBe('100%');
	});

	it('refuses a zoom that would collapse the image', () => {
		expect(frameWidth(0, 2100)).toBe('100%');
		expect(frameWidth(-1, 2100)).toBe('100%');
	});
});

describe('previewScale', () => {
	it('measures fit, since what it came out as is not what it asked for', () => {
		expect(previewScale('fit', 2100, 525)).toBe(0.25);
		expect(previewScale('fit', 2100, 2100)).toBe(1);
	});

	// The bug the measurement exists to catch: fit asks for 100% and gets
	// 536, but a scrollbar means the image actually came out 521. Scaling by
	// the width that was *asked for* draws a grid 3% off the art it
	// describes — most of a square by the far edge of a thirty-square map.
	it('scales fit by what the image came out as, not what it was asked for', () => {
		expect(previewScale('fit', 2100, 521)).toBeCloseTo(521 / 2100, 10);
		expect(previewScale('fit', 2100, 521)).not.toBe(previewScale('fit', 2100, 536));
	});

	// A numeric zoom takes its scale from the number, not from a
	// measurement that arrives a frame later. Passing a stale displayWidth
	// here is the point: it stands for the frame between the click and the
	// observer, when the art has resized and the measurement hasn't caught
	// up. The grid must already be at the new scale.
	it('takes a numeric zoom at its word rather than waiting for a measurement', () => {
		const staleFromFit = 525;
		expect(previewScale(1, 2100, staleFromFit)).toBe(1);
		expect(previewScale(2, 2100, staleFromFit)).toBe(2);
	});

	it('falls back to 1 rather than 0 or NaN when a figure is missing', () => {
		// 0 collapses every length in the overlay; NaN drops the CSS
		// declaration and the grid vanishes with no error anywhere.
		expect(previewScale('fit', 0, 500)).toBe(1);
		expect(previewScale('fit', 2100, 0)).toBe(1);
		expect(previewScale('fit', Number.NaN, 500)).toBe(1);
		expect(previewScale('fit', 2100, Number.NaN)).toBe(1);
		expect(previewScale(0, 2100, 0)).toBe(1);
	});
});
