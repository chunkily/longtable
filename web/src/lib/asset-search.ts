// Narrowing the asset library. The whole library is already in memory —
// the picker and the assets page both load it once — so this is a filter
// over an array rather than a query, and the same function runs in both
// places so they can't drift.
import type { Asset } from './api';

/**
 * Assets whose name or attribution matches every word of the query.
 *
 * Every word rather than the whole string: someone who remembers "goblin"
 * and "archer" shouldn't have to remember which order they typed them in,
 * and an accidental double space shouldn't empty the grid.
 */
export function filterAssets<T extends Pick<Asset, 'name' | 'attribution'>>(
	assets: T[],
	query: string
): T[] {
	const words = query.toLowerCase().split(/\s+/).filter(Boolean);
	if (words.length === 0) return assets;

	return assets.filter((asset) => {
		const haystack = `${asset.name} ${asset.attribution}`.toLowerCase();
		return words.every((word) => haystack.includes(word));
	});
}
