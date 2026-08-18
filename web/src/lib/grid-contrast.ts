// Whether this browser draws the grid in its bold, high-contrast style.
//
// A viewing preference rather than room state, so it is persisted here
// and never sent over the wire — the same call the theme control and the
// GM's fog opacity make. What makes it per-browser rather than per-room
// is that it answers a question about *this screen*: the default grid is
// deliberately faint so it reads as a reference rather than fighting the
// map art, and whether that is too faint depends on the art, the screen
// and the eyes in front of it. Two people at one table can disagree and
// both be right.

const KEY = 'longtable:gridContrast';

export const DEFAULT_HIGH_CONTRAST_GRID = false;

// Stored as a word rather than a bare boolean so the value says what it
// means when somebody is reading their own localStorage, and so an
// unrecognised setting falls back rather than reading as `true` the way
// any non-empty string would.
const BOLD = 'bold';

export function loadHighContrastGrid(): boolean {
	try {
		return localStorage.getItem(KEY) === BOLD;
	} catch {
		return DEFAULT_HIGH_CONTRAST_GRID;
	}
}

export function saveHighContrastGrid(value: boolean) {
	try {
		if (value) {
			localStorage.setItem(KEY, BOLD);
		} else {
			// Removed rather than written as 'faint': the default is off, so
			// an absent key and an explicit "no" mean the same thing, and
			// only one of them survives a change of default.
			localStorage.removeItem(KEY);
		}
	} catch {
		// Private browsing, or storage full. The toggle still works for
		// this page; it simply won't be remembered next time.
	}
}
