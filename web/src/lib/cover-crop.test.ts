import { describe, expect, it } from 'vitest';
import { coverCrop } from './cover-crop';

describe('coverCrop', () => {
	// The playtest case: a 2:3 portrait on a square token keeps its full
	// width and loses equal strips off the top and bottom, rather than being
	// squashed to two-thirds of its height.
	it('crops a portrait to a centred square', () => {
		expect(coverCrop(400, 600, 70, 70)).toEqual({ x: 0, y: 100, width: 400, height: 400 });
	});

	it('crops a landscape to a centred square', () => {
		expect(coverCrop(600, 400, 70, 70)).toEqual({ x: 100, y: 0, width: 400, height: 400 });
	});

	// Square art on a square token is drawn whole, exactly as before.
	it('leaves art that already has the box shape alone', () => {
		expect(coverCrop(512, 512, 140, 140)).toEqual({ x: 0, y: 0, width: 512, height: 512 });
	});

	// The crop is in the image's pixels and only the box's *shape* matters,
	// so a Gargantuan token and a Tiny one take the same region.
	it('depends on the shape of the box, not its size', () => {
		expect(coverCrop(400, 600, 280, 280)).toEqual(coverCrop(400, 600, 35, 35));
	});

	// An image that hasn't decoded reports 0×0; the whole image is the
	// answer that draws what used to be drawn rather than nothing.
	it('falls back to the whole image when anything is zero', () => {
		expect(coverCrop(0, 0, 70, 70)).toEqual({ x: 0, y: 0, width: 0, height: 0 });
		expect(coverCrop(400, 600, 0, 70)).toEqual({ x: 0, y: 0, width: 400, height: 600 });
	});
});
