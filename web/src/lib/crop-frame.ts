// The square a piece of token art is cut to, and how it moves.
//
// Everything here is in the image's *natural* pixels — the units the
// server crops in — and the cropper's display goes through one scale on
// the way to and from the screen. grid-zoom.ts records why the two are
// kept apart: mixing them is the easiest bug to write in this kind of
// component and the hardest to see, since it's right whenever the preview
// happens to be shown at 100%.

export interface CropSquare {
	x: number;
	y: number;
	size: number;
}

/**
 * The smallest square the cropper will zoom to. Small enough to frame a
 * face out of a full-body piece, big enough that what's left is still a
 * picture once it's drawn across a Gargantuan token. An image smaller than
 * this can't be zoomed at all and is framed whole.
 */
export const MIN_CROP_SIZE = 32;

/** Whether this art needs framing at all — a square picture doesn't. */
export function needsCrop(width: number, height: number): boolean {
	return width > 0 && height > 0 && width !== height;
}

/**
 * Where the frame starts: the largest square that fits.
 *
 * Centred across a wide picture, but **anchored to the top** of a tall
 * one, because a tall piece of token art is nearly always a portrait and
 * the face is at the top — centring it frames a belt buckle. The default
 * being right is what lets the cropper be required without being a chore:
 * most of the time it's one click on Add to library.
 */
export function initialCrop(width: number, height: number): CropSquare {
	const size = Math.min(width, height);
	return {
		x: Math.round((width - size) / 2),
		y: 0,
		size
	};
}

/**
 * The square kept wholly inside the picture and no smaller than
 * MIN_CROP_SIZE (or the picture, if that is smaller still). Whole pixels,
 * because the server crops whole pixels and a fraction sent there would be
 * refused.
 */
export function clampCrop(crop: CropSquare, width: number, height: number): CropSquare {
	const largest = Math.min(width, height);
	const size = Math.round(Math.min(largest, Math.max(Math.min(MIN_CROP_SIZE, largest), crop.size)));
	return {
		x: Math.round(Math.min(width - size, Math.max(0, crop.x))),
		y: Math.round(Math.min(height - size, Math.max(0, crop.y))),
		size
	};
}

/** The square moved by (dx, dy) natural pixels, stopping at the edges. */
export function moveCrop(
	crop: CropSquare,
	dx: number,
	dy: number,
	width: number,
	height: number
): CropSquare {
	return clampCrop({ ...crop, x: crop.x + dx, y: crop.y + dy }, width, height);
}

/**
 * The square resized to `size`, keeping its centre where it was — zooming
 * in on what's in the middle of the frame, which is what someone zooming
 * is looking at. Pushed back inside the picture if growing it about that
 * centre would cross an edge.
 */
export function resizeCrop(
	crop: CropSquare,
	size: number,
	width: number,
	height: number
): CropSquare {
	const cx = crop.x + crop.size / 2;
	const cy = crop.y + crop.size / 2;
	return clampCrop({ x: cx - size / 2, y: cy - size / 2, size }, width, height);
}

/**
 * Zoom as the cropper's slider shows it: 1 is the largest square, and
 * higher is closer in. The inverse of `sizeForZoom`.
 */
export function zoomForSize(size: number, width: number, height: number): number {
	return Math.min(width, height) / size;
}

/** The square's side for a given zoom. */
export function sizeForZoom(zoom: number, width: number, height: number): number {
	return Math.min(width, height) / zoom;
}

/** How far the slider zooms in for this picture — never past MIN_CROP_SIZE. */
export function maxZoom(width: number, height: number): number {
	return Math.max(1, Math.min(width, height) / MIN_CROP_SIZE);
}
