// The Host's banner: one message, set when the server was started,
// shown on every page until each person dismisses it.
//
// A module-level singleton rather than per-page state, for two reasons.
// The room page is `fixed inset-0` and paints over anything the layout
// puts above it, so the room's toolbar has to know the banner is there
// to move out of its way — which means something outside both of them
// has to hold the answer. And it is asked for once per page load rather
// than once per component, which a shared instance gets for free.

import { fetchNotice } from './api';

const DISMISSED_KEY = 'longtable:notice-dismissed';

class HostNotice {
	text = $state('');
	/**
	 * The banner's measured height, bound from the element itself rather
	 * than assumed from a class: the one thing that varies is the Host's
	 * sentence, and anything long enough to wrap would otherwise sit over
	 * the top of whatever page is behind it. Read `height`, not this.
	 */
	measured = $state(0);
	// Starts dismissed so nothing flashes on screen before the answer
	// arrives — an empty server is the common case, and a banner that
	// appears for one frame on every page load would be worse than one
	// that appears a moment late.
	dismissed = $state(true);
	private loaded = false;

	get visible(): boolean {
		return this.text !== '' && !this.dismissed;
	}

	/**
	 * How much room to leave at the top of the page, in pixels — 0 when
	 * the banner isn't showing. Derived from `visible` rather than taken
	 * straight from the measurement, because unmounting the element
	 * leaves the last bound value behind: dismissing gave the banner back
	 * its space on screen and kept the gap it had been occupying.
	 */
	get height(): number {
		return this.visible ? this.measured : 0;
	}

	/** Asked once per page load; later callers get the first answer. */
	async load() {
		if (this.loaded) return;
		this.loaded = true;

		const text = await fetchNotice();
		if (!text) return;
		this.text = text;
		// Keyed by the message itself rather than a flag: a Host who
		// changes the banner is saying something new, and it has to come
		// back for everyone who dismissed the last one.
		this.dismissed = read() === text;
	}

	dismiss() {
		this.dismissed = true;
		try {
			localStorage.setItem(DISMISSED_KEY, this.text);
		} catch {
			// Private browsing, or storage full. Dismissing still works for
			// this page; it simply won't be remembered, which is a better
			// outcome than a banner that can't be closed at all.
		}
	}
}

function read(): string | null {
	try {
		return localStorage.getItem(DISMISSED_KEY);
	} catch {
		return null;
	}
}

export const hostNotice = new HostNotice();
