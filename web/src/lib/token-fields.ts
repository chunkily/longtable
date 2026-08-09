// What a token.update carries, and whether two of them are the same.
//
// Three things need this one answer and would otherwise each have their
// own: the edit dialog asking "has anything been typed?" before it warns
// about losing it, `updateToken` asking "is this a no-op?" before it
// sends, and undo asking "is this token still how I left it?" before it
// reverts. Three hand-rolled comparisons is three chances to forget the
// conditions array.

import { tokenTrackers, type Token, type Tracker } from './room.svelte';

/**
 * Every field an edit can change — token.update's payload minus the id.
 * Position isn't here, deliberately, for the same reason it isn't on the
 * wire: moving is its own command, so an edit can't undo a drag.
 */
export interface TokenFields {
	name: string;
	imageAssetId: string | null;
	width: number;
	height: number;
	ownerParticipantId: string | null;
	visibility: 'visible' | 'hidden';
	trackers: Tracker[];
	conditions: string[];
}

/** The token as an edit would describe it. */
export function tokenFields(token: Token): TokenFields {
	return {
		name: token.name,
		imageAssetId: token.imageAssetId,
		width: token.width,
		height: token.height,
		ownerParticipantId: token.ownerParticipantId,
		visibility: token.visibility,
		// Padded to the full three slots, so a token stored before trackers
		// existed compares equal to a form that shows three empty boxes.
		trackers: tokenTrackers(token).map((t) => ({ ...t })),
		conditions: [...(token.conditions ?? [])]
	};
}

/**
 * Whether two sets of fields describe the same token.
 *
 * Order matters for both arrays and that is not an oversight: a tracker
 * slot's identity is its position, and the conditions list is shown in
 * the order it was built, so re-ordering either is a real change
 * somebody made on purpose.
 */
export function sameTokenFields(a: TokenFields, b: TokenFields): boolean {
	return (
		a.name === b.name &&
		a.imageAssetId === b.imageAssetId &&
		a.width === b.width &&
		a.height === b.height &&
		a.ownerParticipantId === b.ownerParticipantId &&
		a.visibility === b.visibility &&
		sameTrackers(a.trackers, b.trackers) &&
		sameConditions(a.conditions, b.conditions)
	);
}

function sameTrackers(a: Tracker[], b: Tracker[]): boolean {
	if (a.length !== b.length) return false;
	// `value` is compared with === on purpose: null is an empty slot and 0
	// is a creature on nought hit points, and the whole nullable value
	// exists to keep those apart. `==` would call them equal.
	return a.every((t, i) => t.label === b[i].label && t.value === b[i].value);
}

function sameConditions(a: string[], b: string[]): boolean {
	return a.length === b.length && a.every((c, i) => c === b[i]);
}
