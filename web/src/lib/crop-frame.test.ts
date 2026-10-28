import { describe, expect, it } from 'vitest';
import {
	MIN_CROP_SIZE,
	clampCrop,
	initialCrop,
	maxZoom,
	moveCrop,
	needsCrop,
	resizeCrop,
	sizeForZoom,
	zoomForSize
} from './crop-frame';

describe('needsCrop', () => {
	it('asks only of art that is not square', () => {
		expect(needsCrop(400, 600)).toBe(true);
		expect(needsCrop(600, 400)).toBe(true);
		expect(needsCrop(512, 512)).toBe(false);
		// Not loaded yet: nothing to ask about.
		expect(needsCrop(0, 0)).toBe(false);
	});
});

describe('initialCrop', () => {
	// A tall piece of token art is a portrait, and centring it frames the
	// middle of the body — the face is at the top.
	it('anchors a portrait to the top', () => {
		expect(initialCrop(400, 600)).toEqual({ x: 0, y: 0, size: 400 });
	});

	it('centres a landscape', () => {
		expect(initialCrop(600, 400)).toEqual({ x: 100, y: 0, size: 400 });
	});

	// The server crops whole pixels, so an odd margin rounds rather than
	// sending half a pixel.
	it('lands on whole pixels', () => {
		expect(initialCrop(601, 400)).toEqual({ x: 101, y: 0, size: 400 });
	});
});

describe('clampCrop', () => {
	it('keeps the square inside the picture', () => {
		expect(clampCrop({ x: -20, y: 500, size: 200 }, 400, 600)).toEqual({ x: 0, y: 400, size: 200 });
	});

	it('never grows past the largest square that fits', () => {
		expect(clampCrop({ x: 0, y: 0, size: 900 }, 400, 600)).toEqual({ x: 0, y: 0, size: 400 });
	});

	it('never shrinks past the smallest useful frame', () => {
		expect(clampCrop({ x: 0, y: 0, size: 3 }, 400, 600).size).toBe(MIN_CROP_SIZE);
	});

	// A picture smaller than the minimum can't satisfy it; framing it whole
	// is the answer that still uploads.
	it('frames a tiny picture whole', () => {
		expect(clampCrop({ x: 0, y: 0, size: 3 }, 20, 30)).toEqual({ x: 0, y: 0, size: 20 });
	});
});

describe('moveCrop', () => {
	it('moves by natural pixels and stops at the edges', () => {
		const crop = { x: 0, y: 0, size: 400 };
		expect(moveCrop(crop, 0, 50, 400, 600)).toEqual({ x: 0, y: 50, size: 400 });
		expect(moveCrop(crop, 0, 5000, 400, 600)).toEqual({ x: 0, y: 200, size: 400 });
		// A full-width square can't move sideways at all.
		expect(moveCrop(crop, 30, 0, 400, 600)).toEqual(crop);
	});
});

describe('resizeCrop', () => {
	// Zooming in on the middle of the frame is zooming in on what's being
	// looked at.
	it('keeps the centre where it was', () => {
		expect(resizeCrop({ x: 0, y: 100, size: 400 }, 200, 400, 600)).toEqual({
			x: 100,
			y: 200,
			size: 200
		});
	});

	// Growing a small square near an edge would cross it about its centre;
	// it's pushed back inside rather than refused.
	it('pushes a growing square back inside', () => {
		expect(resizeCrop({ x: 0, y: 0, size: 100 }, 400, 400, 600)).toEqual({ x: 0, y: 0, size: 400 });
	});
});

describe('zoom', () => {
	it('is 1 at the largest square and rises as the square shrinks', () => {
		expect(zoomForSize(400, 400, 600)).toBe(1);
		expect(zoomForSize(100, 400, 600)).toBe(4);
		expect(sizeForZoom(4, 400, 600)).toBe(100);
	});

	it('stops where the square would pass the minimum', () => {
		expect(maxZoom(400, 600)).toBe(400 / MIN_CROP_SIZE);
		expect(maxZoom(20, 30)).toBe(1);
	});
});
