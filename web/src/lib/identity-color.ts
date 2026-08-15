// The colours a seat can be, and the only place their hex lives.
//
// A seat stores a *key* — `violet`, not `#8b5cf6`. The server validates
// against its own copy of these keys (`IdentityColors` in
// internal/store/participant.go) and never handles a colour, which keeps
// arbitrary text out of the `style` attributes these end up in.
// `TestIdentityColors_MatchTheClientPalette` fails if the two lists drift
// apart, because a key with no hex here renders as no colour at all.
//
// The set dodges the colours the canvas already speaks with — amber is
// the erase highlight, sky blue the measuring tool, red the fog-hide
// preview — and every one has to stay legible on a light map and a dark
// one, which is why they are mid-tone and saturated rather than pastel
// or deep. That constraint is the same one
// planning/backlog/dark-map-stroke-palette.md is still open about for
// drawings.

export interface IdentityColor {
	key: string;
	label: string;
	hex: string;
}

export const IDENTITY_COLORS: IdentityColor[] = [
	{ key: 'violet', label: 'Violet', hex: '#8b5cf6' },
	{ key: 'indigo', label: 'Indigo', hex: '#6366f1' },
	{ key: 'teal', label: 'Teal', hex: '#14b8a6' },
	{ key: 'emerald', label: 'Emerald', hex: '#10b981' },
	{ key: 'lime', label: 'Lime', hex: '#84cc16' },
	{ key: 'pink', label: 'Pink', hex: '#ec4899' }
];

/**
 * The hex for a stored key, or null for a seat that has none.
 *
 * Null rather than a default colour: every seat that predates this
 * feature has an empty key, and those should look the way they always
 * did rather than all turning violet on the same afternoon. Callers
 * decide what "no colour" looks like in their own context.
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
