import { describe, expect, it } from 'vitest';
import { filterAssets } from './asset-search';

const library = [
	{ name: 'Goblin archer', attribution: 'by Alice, CC-BY' },
	{ name: 'Sunless citadel', attribution: '' },
	{ name: 'Tavern interior', attribution: 'art by Bob' }
];

describe('filterAssets', () => {
	it('returns everything for an empty or blank query', () => {
		expect(filterAssets(library, '')).toHaveLength(3);
		expect(filterAssets(library, '   ')).toHaveLength(3);
	});

	it('matches names case-insensitively on a partial word', () => {
		expect(filterAssets(library, 'gob').map((a) => a.name)).toEqual(['Goblin archer']);
		expect(filterAssets(library, 'CITADEL').map((a) => a.name)).toEqual(['Sunless citadel']);
	});

	it('matches attribution text too, so credited art is findable by its source', () => {
		expect(filterAssets(library, 'bob').map((a) => a.name)).toEqual(['Tavern interior']);
		expect(filterAssets(library, 'cc-by').map((a) => a.name)).toEqual(['Goblin archer']);
	});

	// Order shouldn't matter — nobody remembers which way round they typed
	// it — and a stray double space shouldn't empty the grid.
	it('requires every word, in any order and any spacing', () => {
		expect(filterAssets(library, 'archer goblin')).toHaveLength(1);
		expect(filterAssets(library, '  goblin   alice  ')).toHaveLength(1);
		expect(filterAssets(library, 'goblin tavern')).toHaveLength(0);
	});
});
