// Per-room session persistence. There are no accounts — a browser's
// identity in a room is just a session token in localStorage, keyed by
// room slug so the same browser can hold separate identities in
// separate rooms (e.g. GM in one, player in another).
//
// These keys are also the answer to "which rooms am I in", which is what
// the home page lists. Nothing on the server will tell you that: rooms
// aren't enumerable by design, so this is the only record, and it's
// per-browser rather than per-person. See
// planning/user-stories/room-member-sees-their-own-rooms.md.
import type { Session } from './api';

const PREFIX = 'longtable:session:';

/**
 * A session as stored, which is the server's response plus the one thing
 * only this browser knows: when it last opened the room. Used to put the
 * game you're actually playing at the top of the list.
 */
export interface StoredSession extends Session {
	/** ISO timestamp, or absent for a session saved before this existed. */
	lastOpenedAt?: string;
}

function key(slug: string): string {
	return `${PREFIX}${slug}`;
}

export function loadSession(slug: string): Session | null {
	if (typeof window === 'undefined') return null;
	const raw = window.localStorage.getItem(key(slug));
	if (!raw) return null;
	try {
		return JSON.parse(raw) as Session;
	} catch {
		return null;
	}
}

export function saveSession(session: Session): void {
	if (typeof window === 'undefined') return;
	const stored: StoredSession = { ...session, lastOpenedAt: new Date().toISOString() };
	window.localStorage.setItem(key(session.roomSlug), JSON.stringify(stored));
}

/**
 * Marks a room as opened now, so the list orders by when someone last
 * played rather than when they first joined.
 *
 * Separate from `loadSession` on purpose: that runs in places where
 * merely reading the session isn't the same as sitting down at the table
 * — the assets page is one — and having a read quietly reorder the home
 * page would be a surprise.
 */
export function touchSession(slug: string): void {
	if (typeof window === 'undefined') return;
	const existing = loadSession(slug);
	if (!existing) return;
	saveSession(existing);
}

export function clearSession(slug: string): void {
	if (typeof window === 'undefined') return;
	window.localStorage.removeItem(key(slug));
}

/**
 * Every room this browser has joined or created, most recently opened
 * first.
 *
 * Anything unparseable is skipped rather than thrown on: this reads keys
 * that could have been written by an older version of the app, and one
 * bad entry shouldn't cost someone the rest of their list.
 */
export function listSessions(): StoredSession[] {
	if (typeof window === 'undefined') return [];

	const sessions: StoredSession[] = [];
	for (let i = 0; i < window.localStorage.length; i++) {
		const storageKey = window.localStorage.key(i);
		if (!storageKey?.startsWith(PREFIX)) continue;
		const raw = window.localStorage.getItem(storageKey);
		if (!raw) continue;
		try {
			const parsed = JSON.parse(raw) as StoredSession;
			// A key with no slug behind it can't be linked to, so it's no
			// use in a list of places to go back to.
			if (parsed?.roomSlug) sessions.push(parsed);
		} catch {
			continue;
		}
	}

	// Sessions saved before lastOpenedAt existed sort last rather than
	// first — they're the oldest thing this browser knows either way.
	return sessions.sort((a, b) => (b.lastOpenedAt ?? '').localeCompare(a.lastOpenedAt ?? ''));
}
