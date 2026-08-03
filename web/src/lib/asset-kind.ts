// Reading an image's shape to say whether it looks like token art or a
// battle map. Used to *question* a choice someone already made, never to
// make it for them — see the assets page.
import type { AssetKind } from './api';

/**
 * The widest a token's art gets before it stops looking like a token.
 * Square art much bigger than this is more likely a map that happens to
 * be square than a portrait nobody will ever see at full size.
 */
const MAX_TOKEN_EDGE = 1200;

/**
 * How far from square art can be and still read as a token. Token art is
 * square by convention — it ends up in a circular crop on a square
 * grid cell — so the ratio is the strongest signal there is here.
 */
const MIN_TOKEN_RATIO = 0.8;
const MAX_TOKEN_RATIO = 1.25;

/**
 * What an image of these dimensions probably is.
 *
 * Squareness does most of the work, and size only breaks the tie for
 * large square images. Sizing alone is what you reach for first and it
 * doesn't survive contact with real art: token art is routinely
 * 256–1024px, which is comfortably "large", so any plain pixel threshold
 * low enough to catch maps calls almost every token a map too.
 *
 * Wrong often enough that nothing should act on it silently. It's a
 * prompt, not a classifier.
 */
export function guessAssetKind(width: number, height: number): AssetKind {
	if (width <= 0 || height <= 0) return 'token';

	const ratio = width / height;
	const squarish = ratio >= MIN_TOKEN_RATIO && ratio <= MAX_TOKEN_RATIO;
	const modest = Math.max(width, height) <= MAX_TOKEN_EDGE;

	return squarish && modest ? 'token' : 'map';
}

/**
 * Reads an image's natural dimensions from a URL — an object URL, in
 * practice, for a file that hasn't been uploaded yet.
 *
 * Resolves to null rather than rejecting when the image won't load: a
 * shape nobody could measure is one nobody should be warned about, and
 * the upload itself is still perfectly allowed to proceed and fail on
 * the server, which is where the real format check lives.
 */
export function measureImage(url: string): Promise<{ width: number; height: number } | null> {
	return new Promise((resolve) => {
		const img = new Image();
		img.onload = () => resolve({ width: img.naturalWidth, height: img.naturalHeight });
		img.onerror = () => resolve(null);
		img.src = url;
	});
}
