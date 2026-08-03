import { describe, expect, it } from 'vitest';
import { ownerOptions } from './token-owner';
import type { Participant } from './room.svelte';

const alice: Participant = { id: 'p1', displayName: 'Alice', role: 'gm' };
const bob: Participant = { id: 'p2', displayName: 'Bob', role: 'player' };
// Joined once weeks ago and never came back — the kind of entry that
// makes the full roster the wrong list to choose from.
const carol: Participant = { id: 'p3', displayName: 'Carol', role: 'player' };

describe('ownerOptions', () => {
	it('offers the people at the table, not everyone who has ever joined', () => {
		const options = ownerOptions([alice, bob], [alice, bob, carol], null);

		expect(options.map((o) => o.participant.displayName)).toEqual(['Alice', 'Bob']);
		expect(options.every((o) => o.online)).toBe(true);
	});

	// The one that matters: token.update sends the owner every time, so an
	// absent owner missing from the list would be silently unassigned by
	// an edit that only meant to rename the token.
	it('keeps a token’s current owner on the list once they go offline', () => {
		const options = ownerOptions([alice], [alice, bob], bob.id);

		expect(options.map((o) => o.participant.displayName)).toEqual(['Alice', 'Bob']);
		expect(options.find((o) => o.participant.id === bob.id)?.online).toBe(false);
	});

	it('does not list a connected owner twice', () => {
		const options = ownerOptions([alice, bob], [alice, bob], bob.id);

		expect(options.map((o) => o.participant.id)).toEqual(['p1', 'p2']);
	});

	// Nothing can name them, so an option for them would be a blank line.
	// The server would still accept the id — the details panel says what
	// it can, and this list stays honest about who it can offer.
	it('leaves out an owner who is in neither list', () => {
		const options = ownerOptions([alice], [alice], 'ghost');

		expect(options.map((o) => o.participant.id)).toEqual(['p1']);
	});

	it('offers nobody at all when nobody is connected and the token is unowned', () => {
		expect(ownerOptions([], [alice, bob], null)).toEqual([]);
	});
});
