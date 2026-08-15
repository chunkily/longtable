import { describe, expect, it } from 'vitest';
import { DRAWING_STROKE_WIDTH } from './drawing-hit';
import { MAX_STROKE_WIDTH, MIN_STROKE_WIDTH, STROKE_WIDTH_CHOICES } from './stroke-width';

describe('STROKE_WIDTH_CHOICES', () => {
	// A width outside the server's clamp comes back as a different number,
	// so the stroke on the map is not the one the button said it would be
	// — and the round trip is silent about having changed it.
	it('offers only widths the server accepts unchanged', () => {
		for (const choice of STROKE_WIDTH_CHOICES) {
			expect(choice.value).toBeGreaterThanOrEqual(MIN_STROKE_WIDTH);
			expect(choice.value).toBeLessThanOrEqual(MAX_STROKE_WIDTH);
		}
	});

	// Nothing picks a width until someone clicks one, so a default that
	// isn't on the strip leaves it open with no button pressed.
	it('includes the default width', () => {
		expect(STROKE_WIDTH_CHOICES.map((choice) => choice.value)).toContain(DRAWING_STROKE_WIDTH);
	});

	it('ascends, so the buttons read thin to thick from left to right', () => {
		const values = STROKE_WIDTH_CHOICES.map((choice) => choice.value);
		expect(values).toEqual([...values].sort((a, b) => a - b));
		expect(new Set(values).size).toBe(values.length);
	});

	// The bar is drawn at a fixed size in the button, so two choices that
	// looked different on the wire but the same on screen would give the
	// strip two identical-looking buttons.
	it('draws a different bar for each width', () => {
		const bars = STROKE_WIDTH_CHOICES.map((choice) => choice.bar);
		expect(new Set(bars).size).toBe(bars.length);
	});
});
