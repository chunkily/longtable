// Who a token can be handed to: the Players at the table.
//
// Two narrowings, each with a reason worth keeping.
//
// Not the room's roster, because a participant row is created on every
// join — the same person turns up again from a phone, or after clearing
// their browser storage, and a room that has run for a few months lists
// a dozen people to choose four from. Who is actually here is the
// question a GM is answering.
//
// And not the GMs among them, because a GM owning a token would grant
// nothing: a GM may move any token whatever the ownership lock says, and
// may already edit any token's HP. The only thing it could express is
// "this is my character", and a choice that looks like a permission but
// isn't is worse than not offering it.
import type { Participant } from './room.svelte';

export interface OwnerOption {
	participant: Participant;
	/**
	 * Whether they're connected right now — not whether they were offered
	 * or merely kept. A GM who owns a token is kept on the list while
	 * sitting right there, and labelling them absent would be a lie.
	 */
	online: boolean;
}

/**
 * The people a token may be assigned to: the connected Players, plus its
 * current owner whoever that turns out to be.
 *
 * That second half is not a nicety. `token.update` sends every editable
 * field every time, so a list that dropped the current owner would leave
 * their name missing from the select, the browser would fall back to the
 * first option — "Nobody" — and saving an unrelated change to the
 * token's name would quietly take it away from them. It catches both
 * ways of falling off the list: a Player who has gone offline, and a GM
 * who owns a token from before GMs stopped being offered.
 *
 * An owner in neither list is left out entirely: they can't be named, so
 * offering them would mean an option with no label. The details panel
 * handles that case separately, and the server would still accept them —
 * it checks membership of the room, not presence or role.
 */
export function ownerOptions(
	connected: Participant[],
	roster: Participant[],
	ownerId: string | null
): OwnerOption[] {
	const options: OwnerOption[] = connected
		.filter((p) => p.role !== 'gm')
		.map((participant) => ({ participant, online: true }));
	if (ownerId === null || options.some((o) => o.participant.id === ownerId)) {
		return options;
	}

	// Looked up in the roster rather than in `connected`, so this covers
	// an owner who left as well as one who is here but no longer offered.
	const kept = roster.find((p) => p.id === ownerId);
	if (!kept) return options;

	// Last, after the people who are here — it's the exception, and a
	// select shows whichever option is chosen regardless of position.
	return [...options, { participant: kept, online: connected.some((p) => p.id === ownerId) }];
}
