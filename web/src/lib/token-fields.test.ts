import { describe, expect, it } from 'vitest';
import { sameTokenFields, tokenFields, type TokenFields } from './token-fields';
import type { Token } from './room.svelte';

// Three callers lean on this being exactly right: the dialog's "warn me
// before I lose this", updateToken's "don't send a no-op", and undo's
// "is this token still how I left it". A comparison that missed a field
// would make all three quietly wrong in different ways.

const goblin: Token = {
	id: 't1',
	sceneId: 's1',
	name: 'Goblin',
	imageAssetId: null,
	x: 3,
	y: 4,
	width: 1,
	height: 1,
	ownerParticipantId: null,
	visibility: 'visible',
	trackers: [{ label: 'HP', value: 7 }],
	conditions: ['Prone']
};

describe('tokenFields', () => {
	it('pads the trackers to the full three slots', () => {
		// A token stored before trackers existed, or one the server hasn't
		// normalised yet, has to compare equal to a form showing three
		// boxes — otherwise every such token reads as dirty on open.
		expect(tokenFields(goblin).trackers).toEqual([
			{ label: 'HP', value: 7 },
			{ label: '', value: null },
			{ label: '', value: null }
		]);
	});

	it('leaves position out, since a drag is not an edit', () => {
		expect(tokenFields(goblin)).not.toHaveProperty('x');
		expect(tokenFields(goblin)).not.toHaveProperty('y');
	});

	it('copies the arrays rather than sharing them with the token', () => {
		const fields = tokenFields(goblin);
		fields.conditions.push('Poisoned');
		fields.trackers[0].value = 1;

		expect(goblin.conditions).toEqual(['Prone']);
		expect(goblin.trackers?.[0].value).toBe(7);
	});
});

describe('sameTokenFields', () => {
	const base = tokenFields(goblin);
	const changed = (patch: Partial<TokenFields>): TokenFields => ({ ...base, ...patch });

	it('is true for an untouched copy', () => {
		expect(sameTokenFields(base, tokenFields(goblin))).toBe(true);
	});

	it.each([
		['name', { name: 'Hobgoblin' }],
		['art', { imageAssetId: 'asset-1' }],
		['width', { width: 2 }],
		['height', { height: 2 }],
		['owner', { ownerParticipantId: 'p2' }],
		['visibility', { visibility: 'hidden' as const }],
		['conditions', { conditions: ['Prone', 'Poisoned'] }]
	])('notices a change of %s', (_what, patch) => {
		expect(sameTokenFields(base, changed(patch))).toBe(false);
	});

	it('notices a tracker value, a label, and a cleared slot', () => {
		const value = changed({ trackers: [{ label: 'HP', value: 6 }, ...base.trackers.slice(1)] });
		const label = changed({ trackers: [{ label: 'Hits', value: 7 }, ...base.trackers.slice(1)] });
		const cleared = changed({
			trackers: [{ label: 'HP', value: null }, ...base.trackers.slice(1)]
		});

		expect(sameTokenFields(base, value)).toBe(false);
		expect(sameTokenFields(base, label)).toBe(false);
		expect(sameTokenFields(base, cleared)).toBe(false);
	});

	// The distinction the nullable value exists for: an empty slot and a
	// creature on nought hit points are different states, and a loose
	// comparison would call them the same and skip the send.
	it('does not confuse an empty slot with a zero', () => {
		const zero = changed({ trackers: [{ label: 'HP', value: 0 }, ...base.trackers.slice(1)] });
		const empty = changed({ trackers: [{ label: 'HP', value: null }, ...base.trackers.slice(1)] });

		expect(sameTokenFields(zero, empty)).toBe(false);
	});

	// Both lists are shown in the order they're stored, so re-ordering one
	// is a change somebody made deliberately.
	it('treats a reordered condition list as a change', () => {
		const reordered = changed({ conditions: ['Poisoned', 'Prone'] });
		const original = changed({ conditions: ['Prone', 'Poisoned'] });

		expect(sameTokenFields(original, reordered)).toBe(false);
	});
});
