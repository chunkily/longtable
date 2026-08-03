// Who a token can be handed to.
//
// Not simply "the room's roster": a participant row is created on every
// join, so the same person turns up again from a phone, or after
// clearing their browser storage, and a room that has run for a few
// months lists a dozen people to choose four from. Who is actually at
// the table is the useful question.
import type { Participant } from './room.svelte';

export interface OwnerOption {
	participant: Participant;
	/**
	 * False for someone on the list only because they already own the
	 * token. Worth marking, since "not here right now" is the reason
	 * they'd otherwise be missing.
	 */
	online: boolean;
}

/**
 * The people a token may be assigned to: everyone connected, plus its
 * current owner if they've since gone offline.
 *
 * That second half is not a nicety. `token.update` sends every editable
 * field every time, so a list that dropped an absent owner would leave
 * their name missing from the select, the browser would fall back to the
 * first option — "Nobody" — and saving an unrelated change to the
 * token's name would quietly take it away from them.
 *
 * An owner who is in neither list is left out entirely: they can't be
 * named, so offering them would mean an option with no label. The
 * details panel handles that case separately, and the server would still
 * accept them, since being offline is not the same as being gone.
 */
export function ownerOptions(
	connected: Participant[],
	roster: Participant[],
	ownerId: string | null
): OwnerOption[] {
	const options: OwnerOption[] = connected.map((participant) => ({ participant, online: true }));
	if (ownerId === null || options.some((o) => o.participant.id === ownerId)) {
		return options;
	}

	const absent = roster.find((p) => p.id === ownerId);
	// Last, after the people who are here — it's the exception, and a
	// select shows whichever option is chosen regardless of position.
	return absent ? [...options, { participant: absent, online: false }] : options;
}
