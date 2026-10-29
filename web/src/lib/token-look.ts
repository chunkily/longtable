// What a token without art looks like: the colour of its disc and the
// ink of whatever is drawn on it — an icon, or its initials.
//
// One answer for the map and for every DOM view of a token (the initiative
// panel, the details panel, the hover card, the dialogs), so a goblin is
// the same green everywhere it appears. game-canvas.svelte used to work
// the hash colour out for itself and always draw white on it, which
// vanished on the yellow half of the wheel.
import { IDENTITY_COLORS } from './identity-color';

/** The two inks, chosen per disc colour for contrast. */
export const INK = { dark: '#18181b', light: '#ffffff' } as const;

/**
 * The two-tone ring round a disc token on the map: a dark band outside a
 * pale one. The bold grid's answer to the same problem (GRID_LINE_BOLD in
 * game-canvas.svelte) — a disc has to stand out against map art that has
 * no idea what colour it will be, and one ring colour only moves the
 * failure (white on a snow map, black on a dungeon floor). The same pair
 * whatever the disc colour and the page's scheme: the art is what it's
 * set against, not the page.
 */
export const DISC_RING = { outer: '#18181be6', inner: '#ffffffd9' } as const;

export interface DiscLook {
	fill: string;
	ink: string;
}

/**
 * A token's disc: its chosen colour if it has one, otherwise the palette
 * colour its id hashes to.
 *
 * The fallback used to be a hue anywhere on the wheel, so a token left on
 * `Any` came out in a colour none of the sixteen swatches showed — and
 * one nobody could pick to match it. It is a palette colour now, with that
 * colour's stored ink. Tokens from before that changed colour once
 * (2026-10-29); a deliberate break while there was one table to break.
 */
export function discLook(token: { id: string; color?: string | null }): DiscLook {
	const chosen = token.color ? IDENTITY_COLORS.find((c) => c.key === token.color) : undefined;
	const c = chosen ?? IDENTITY_COLORS[idHash(token.id) % IDENTITY_COLORS.length];
	return { fill: c.hex, ink: INK[c.ink] };
}

/** A stable hash of an id — what picks an unchosen token's colour. */
export function idHash(id: string): number {
	let hash = 0;
	for (let i = 0; i < id.length; i++) hash = (hash * 31 + id.charCodeAt(i)) >>> 0;
	return hash;
}

/** The first two letters of a name, as a disc without an icon shows them. */
export function initials(name: string): string {
	return name.slice(0, 2).toUpperCase();
}

/** Whichever ink has the higher WCAG contrast against `rgb` (0-255). */
export function betterInk(rgb: [number, number, number]): 'dark' | 'light' {
	const fill = luminance(rgb);
	const onLight = contrast(fill, luminance(hexRgb(INK.light)));
	const onDark = contrast(fill, luminance(hexRgb(INK.dark)));
	return onLight >= onDark ? 'light' : 'dark';
}

export function hexRgb(hex: string): [number, number, number] {
	const n = parseInt(hex.slice(1, 7), 16);
	return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

function luminance([r, g, b]: [number, number, number]): number {
	const lin = (v: number) => {
		const c = v / 255;
		return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
	};
	return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

function contrast(a: number, b: number): number {
	const [hi, lo] = a > b ? [a, b] : [b, a];
	return (hi + 0.05) / (lo + 0.05);
}
