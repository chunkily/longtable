// The GM's preferred fog cover opacity — how dark fog looks on their own
// screen, not room state, so it's persisted per browser like the theme
// control rather than sent over the wire. Different maps need different
// amounts of contrast against their own art; see
// planning/backlog/fog-gm-view-contrast.md for why a single fixed value
// doesn't work for everyone.

const KEY = 'longtable:fogOpacity';

export const DEFAULT_FOG_OPACITY = 0.5;

// Floored well above 0 — an opacity a GM can't see is indistinguishable
// from no fog at all, defeating the point of the control. Capped short
// of 1 for the opposite reason: "GM sees everything" is the invariant
// fog painting depends on, and a fully opaque cover would hide the map
// from the one person who's meant to always see it.
export const MIN_FOG_OPACITY = 0.15;
export const MAX_FOG_OPACITY = 0.9;

export function loadFogOpacity(): number {
	try {
		const value = Number(localStorage.getItem(KEY));
		return Number.isFinite(value) && value >= MIN_FOG_OPACITY && value <= MAX_FOG_OPACITY
			? value
			: DEFAULT_FOG_OPACITY;
	} catch {
		return DEFAULT_FOG_OPACITY;
	}
}

export function saveFogOpacity(value: number) {
	try {
		localStorage.setItem(KEY, String(value));
	} catch {
		// Private browsing, or storage full. The slider still works for
		// this page; it simply won't be remembered next time.
	}
}
