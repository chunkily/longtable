import { describe, expect, it } from 'vitest';
import { ownerOptions } from './token-owner';
import type { Participant } from './room.svelte';

const alice: Participant = { id: 'p1', displayName: 'Alice', color: '', role: 'gm' };
const bob: Participant = { id: 'p2', displayName: 'Bob', color: '', role: 'player' };
// Joined once weeks ago and never came back — the kind of entry that
// makes the full roster the wrong list to choose from.
const carol: Participant = { id: 'p3', displayName: 'Carol', color: '', role: 'player' };
const dave: Participant = { id: 'p4', displayName: 'Dave', color: '', role: 'player' };

describe('ownerOptions', () => {
	it('offers the people at the table, not everyone who has ever joined', () => {
		const options = ownerOptions([alice, bob, dave], [alice, bob, carol, dave], null);

		expect(options.map((o) => o.participant.displayName)).toEqual(['Bob', 'Dave']);
		expect(options.every((o) => o.online)).toBe(true);
	});

	// Ownership can only ever grant a GM what a GM already has: they may
	// move any token whatever the lock says, and edit any token's HP. An
	// option that looks like a permission and isn't is worse than no
	// option.
	it('does not offer GMs, who would gain nothing by owning a token', () => {
		const options = ownerOptions([alice, bob], [alice, bob], null);

		expect(options.map((o) => o.participant.displayName)).toEqual(['Bob']);
	});

	// Tokens assigned to a GM before that rule existed still have to be
	// editable without the save quietly reassigning them.
	it('keeps a GM who already owns the token, and does not call them absent', () => {
		const options = ownerOptions([alice, bob], [alice, bob], alice.id);

		expect(options.map((o) => o.participant.displayName)).toEqual(['Bob', 'Alice']);
		expect(options.find((o) => o.participant.id === alice.id)?.online).toBe(true);
	});

	// A room of co-GMs before any player arrives. The picker has nothing
	// to offer, which the component says out loud rather than showing an
	// empty control.
	it('offers nobody when only GMs are connected', () => {
		expect(ownerOptions([alice], [alice, bob], null)).toEqual([]);
	});

	// The one that matters: token.update sends the owner every time, so an
	// absent owner missing from the list would be silently unassigned by
	// an edit that only meant to rename the token.
	it('keeps a token’s current owner on the list once they go offline', () => {
		const options = ownerOptions([alice, dave], [alice, bob, dave], bob.id);

		expect(options.map((o) => o.participant.displayName)).toEqual(['Dave', 'Bob']);
		expect(options.find((o) => o.participant.id === bob.id)?.online).toBe(false);
	});

	it('does not list a connected owner twice', () => {
		const options = ownerOptions([bob, dave], [bob, dave], bob.id);

		expect(options.map((o) => o.participant.id)).toEqual(['p2', 'p4']);
	});

	// Nothing can name them, so an option for them would be a blank line.
	// The server would still accept the id — the details panel says what
	// it can, and this list stays honest about who it can offer.
	it('leaves out an owner who is in neither list', () => {
		const options = ownerOptions([bob], [bob], 'ghost');

		expect(options.map((o) => o.participant.id)).toEqual(['p2']);
	});

	it('offers nobody at all when nobody is connected and the token is unowned', () => {
		expect(ownerOptions([], [alice, bob], null)).toEqual([]);
	});
});
