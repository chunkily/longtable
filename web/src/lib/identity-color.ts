// The colours a seat can be, and the only place their hex lives.
//
// Sixteen of them are the Players'. The GM's is a seventeenth that isn't
// in the list and isn't picked — see GM_IDENTITY_COLOR below.
//
// A seat stores a *key* — `violet`, not `#8b5cf6` and not `Arcane Violet`. The server validates
// against its own copy of these keys (`IdentityColors` in
// internal/store/participant.go) and never handles a colour, which keeps
// arbitrary text out of the `style` attributes these end up in.
// `TestIdentityColors_MatchTheClientPalette` fails if the two lists drift
// apart, because a key with no hex here renders as no colour at all.
//
// # Sixteen, and no longer dodging anything
//
// This started as six, chosen to stay clear of the colours the canvas
// already speaks with — amber is the erase highlight, sky blue the
// measuring tool, red the fog-hide preview — and to stay legible on a
// light map and a dark one. Both constraints were dropped on the GM's
// call: a table of six is a table where two people are the same colour
// by the fourth arrival, and neither clash is actually ambiguous in
// practice. A ping is a pulsing ring where the erase halo is a static
// outline on a stroke; a crimson ping is not a fog preview, which only
// ever appears under the GM's own cursor mid-drag.
//
// So the rule now is only that they look good together and that the
// names are worth reading. Contrast against a particular map is the
// chooser's problem, which is the same trade the drawing palette makes.
//
// Ordered around the wheel — warm, green, blue, violet, then the three
// quiet ones — so the picker reads as a spectrum rather than a bag of
// swatches, and `suggestedColor` walking it in order hands out colours
// that look nothing like each other.
//
// Named `<modifier> <colour>` throughout. The modifier is what makes one
// of sixteen sayable out loud at a table — "I'm the blue one" stops
// working at the second blue — and the base colour is what keeps the
// name honest about what you are about to see.
//
// **The key is the base colour alone**, which is the half that doesn't
// change. Renaming Blood Red to something better is then a one-word edit
// here, with every seat already holding `red` unaffected; had the key
// been the whole name, the same edit would orphan everybody wearing it.
// That is also why Swamp Olive is an olive rather than a second green:
// two greens would have collided on one key, and a key collision is the
// one thing this scheme can't absorb.

export interface IdentityColor {
	key: string;
	label: string;
	hex: string;
}

export const IDENTITY_COLORS: IdentityColor[] = [
	{ key: 'red', label: 'Blood Red', hex: '#dc2626' },
	{ key: 'orange', label: 'Sunset Orange', hex: '#f97316' },
	{ key: 'gold', label: 'Honey Gold', hex: '#f59e0b' },
	{ key: 'yellow', label: 'Lantern Yellow', hex: '#eab308' },
	{ key: 'olive', label: 'Swamp Olive', hex: '#65a30d' },
	{ key: 'green', label: 'Forest Green', hex: '#10b981' },
	{ key: 'teal', label: 'Lagoon Teal', hex: '#2dd4bf' },
	{ key: 'cyan', label: 'Frost Cyan', hex: '#06b6d4' },
	{ key: 'blue', label: 'Royal Blue', hex: '#2563eb' },
	{ key: 'indigo', label: 'Midnight Indigo', hex: '#4f46e5' },
	{ key: 'violet', label: 'Arcane Violet', hex: '#8b5cf6' },
	{ key: 'magenta', label: 'Dusk Magenta', hex: '#c026d3' },
	{ key: 'pink', label: 'Rose Pink', hex: '#ec4899' },
	{ key: 'brown', label: 'Rust Brown', hex: '#b45309' },
	{ key: 'grey', label: 'Storm Grey', hex: '#64748b' },
	{ key: 'white', label: 'Bone White', hex: '#e7e5e4' }
];

/**
 * The GM's colour, which is not one of the sixteen and is not chosen:
 * black, with white standing in for it on the dark scheme.
 *
 * Fixed rather than picked because there is exactly one GM in a room and
 * everyone knows which name is theirs — a colour is for telling six
 * players apart, and spending one of the sixteen on the person who needs
 * it least made the palette shorter for everybody else.
 *
 * A pair rather than a hex for the reason black is always a pair here: the
 * GM's name in chat is DOM text on a themed panel, and one that reads on
 * the light scheme is invisible on the dark one. Same two hexes as the
 * drawing palette's black/white column, which is the pairing this repo
 * already speaks.
 *
 * The **ping** is the one place the scheme is a guess: a ping is painted
 * on map art, which has no idea what the page is wearing (the note in
 * stroke-colors.ts is the long version). A GM pinging a dark map under a
 * light UI gets a black ring on dark art. Accepted rather than solved —
 * the alternative is a halo behind every ping, which is a bigger change
 * than the colour it would be fixing.
 */
export const GM_IDENTITY_COLOR = { light: '#000000', dark: '#ffffff' };

export type ColorScheme = 'light' | 'dark';

/**
 * What a seat is painted in: whatever they picked for a Player, and the
 * fixed pair above for the GM, whose stored key is ignored.
 *
 * Ignored rather than migrated, so a room made before this reads the new
 * way without anything having to run over its database — and so there is
 * one answer to "what colour is the GM" rather than a rule and a stored
 * value that can disagree.
 */
export function seatHex(seat: SeatIdentity | null | undefined, scheme: ColorScheme): string | null {
	if (!seat) return null;
	if (seat.role === 'gm') return GM_IDENTITY_COLOR[scheme];
	return identityHex(seat.color);
}

/** The half of a seat this module needs: what they are, and what they picked. */
export interface SeatIdentity {
	role: string;
	color?: string | null;
}

/**
 * The hex for a stored key, or null for a seat that has none.
 *
 * Null rather than a default colour: every seat that predates this
 * feature has an empty key, and those should look the way they always
 * did rather than all turning blood red on the same afternoon. Callers
 * decide what "no colour" looks like in their own context.
 *
 * A key that isn't in the list any more reads as null too, which is what
 * makes retiring a colour safe: the seat goes back to looking unchosen
 * rather than rendering a broken style attribute, and the next thing
 * that seat picks fixes it.
 */
export function identityHex(key: string | null | undefined): string | null {
	return IDENTITY_COLORS.find((c) => c.key === key)?.hex ?? null;
}

/**
 * What to offer somebody who hasn't chosen yet: the first colour nobody
 * at the table is using, or the first in the palette once they are all
 * spoken for.
 *
 * A suggestion, not a rule. Two people may end up the same colour and
 * nothing stops them — the picker shows what is taken so a clash can be
 * avoided by whoever cares, which is a different thing from the room
 * refusing one.
 */
export function suggestedColor(taken: readonly (string | null | undefined)[]): string {
	const used = new Set(taken.filter(Boolean));
	return (IDENTITY_COLORS.find((c) => !used.has(c.key)) ?? IDENTITY_COLORS[0]).key;
}
