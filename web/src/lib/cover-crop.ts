// Which part of a picture fills a box without stretching it.
//
// Token art used to be drawn at exactly the token's width and height, so
// anything not already square was scaled unevenly to fit — a 2:3 portrait
// came out at two-thirds of its real height, and that squashing was the
// first thing people said about token art at a playtest. This is the CSS
// `object-fit: cover` answer instead, which the initiative panel already
// gives the same art: the largest region of the source with the box's
// shape, centred, scaled evenly to fill it. The edges that don't fit are
// cut rather than squeezed.
//
// In the source image's own pixels, because that is what Konva's `crop`
// takes — and keeping natural and display pixels apart is the thing
// grid-zoom.ts warns is easiest to get wrong.

export interface CropRect {
	x: number;
	y: number;
	width: number;
	height: number;
}

/**
 * The centred region of a `naturalWidth`×`naturalHeight` image with the
 * same shape as a `boxWidth`×`boxHeight` box.
 *
 * Anything degenerate — an image that hasn't decoded and reports zero, a
 * zero-sized box — gets the whole image back, which draws exactly what was
 * drawn before this existed rather than nothing at all.
 */
export function coverCrop(
	naturalWidth: number,
	naturalHeight: number,
	boxWidth: number,
	boxHeight: number
): CropRect {
	if (naturalWidth <= 0 || naturalHeight <= 0 || boxWidth <= 0 || boxHeight <= 0) {
		return { x: 0, y: 0, width: Math.max(0, naturalWidth), height: Math.max(0, naturalHeight) };
	}
	const boxRatio = boxWidth / boxHeight;
	const imageRatio = naturalWidth / naturalHeight;
	if (imageRatio > boxRatio) {
		// Wider than the box: full height, the middle of the width.
		const width = naturalHeight * boxRatio;
		return { x: (naturalWidth - width) / 2, y: 0, width, height: naturalHeight };
	}
	// Taller than the box (or the same shape): full width, the middle of the height.
	const height = naturalWidth / boxRatio;
	return { x: 0, y: (naturalHeight - height) / 2, width: naturalWidth, height };
}
