// Per-room session persistence. There are no accounts — a browser's
// identity in a room is just a session token in localStorage, keyed by
// room slug so the same browser can hold separate identities in
// separate rooms (e.g. GM in one, player in another).
import type { Session } from './api';

function key(slug: string): string {
	return `longtable:session:${slug}`;
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
	window.localStorage.setItem(key(session.roomSlug), JSON.stringify(session));
}

export function clearSession(slug: string): void {
	if (typeof window === 'undefined') return;
	window.localStorage.removeItem(key(slug));
}
