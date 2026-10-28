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
 * A token's disc: its chosen colour if it has one, otherwise the colour
 * its id hashes to — the same one it has always had, so tokens from before
 * colours could be chosen don't change.
 */
export function discLook(token: { id: string; color?: string | null }): DiscLook {
	const chosen = token.color ? IDENTITY_COLORS.find((c) => c.key === token.color) : undefined;
	if (chosen) return { fill: chosen.hex, ink: INK[chosen.ink] };
	const [r, g, b] = hslToRgb(idHue(token.id), 0.65, 0.55);
	return {
		fill: `rgb(${r}, ${g}, ${b})`,
		ink: INK[betterInk([r, g, b])]
	};
}

/** The id's hue, 0-359 — what `colorFor` in the canvas always computed. */
export function idHue(id: string): number {
	let hash = 0;
	for (let i = 0; i < id.length; i++) hash = (hash * 31 + id.charCodeAt(i)) >>> 0;
	return hash % 360;
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

function hslToRgb(h: number, s: number, l: number): [number, number, number] {
	const k = (n: number) => (n + h / 30) % 12;
	const a = s * Math.min(l, 1 - l);
	const f = (n: number) => l - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)));
	return [Math.round(f(0) * 255), Math.round(f(8) * 255), Math.round(f(4) * 255)];
}
