import { beforeEach, describe, expect, it } from 'vitest';
import { clearSession, listSessions, loadSession, saveSession, touchSession } from './session';
import type { Session } from './api';

function session(slug: string, overrides: Partial<Session> = {}): Session {
	return {
		roomSlug: slug,
		roomName: `Room ${slug}`,
		participantId: `p-${slug}`,
		displayName: 'Alice',
		role: 'player',
		sessionToken: `t-${slug}`,
		...overrides
	};
}

describe('session storage', () => {
	beforeEach(() => window.localStorage.clear());

	it('round-trips a session by slug, keeping identities in separate rooms apart', () => {
		saveSession(session('aaaaaa', { role: 'gm', displayName: 'Alice' }));
		saveSession(session('bbbbbb', { role: 'player', displayName: 'Bob' }));

		expect(loadSession('aaaaaa')?.role).toBe('gm');
		expect(loadSession('bbbbbb')?.displayName).toBe('Bob');
		expect(loadSession('cccccc')).toBeNull();
	});

	it('lists every room this browser is in', () => {
		saveSession(session('aaaaaa'));
		saveSession(session('bbbbbb'));

		expect(
			listSessions()
				.map((s) => s.roomSlug)
				.sort()
		).toEqual(['aaaaaa', 'bbbbbb']);
	});

	// The game you're actually playing belongs at the top, which is what
	// makes the list useful once a group has a few rooms.
	it('orders by when the room was last opened, not when it was joined', async () => {
		saveSession(session('aaaaaa'));
		await new Promise((r) => setTimeout(r, 2));
		saveSession(session('bbbbbb'));

		expect(listSessions()[0].roomSlug).toBe('bbbbbb');

		await new Promise((r) => setTimeout(r, 2));
		touchSession('aaaaaa');

		expect(listSessions()[0].roomSlug).toBe('aaaaaa');
	});

	it('leaves everything else about a session alone when touching it', () => {
		saveSession(session('aaaaaa', { role: 'gm', sessionToken: 'keep-me' }));
		touchSession('aaaaaa');

		const after = loadSession('aaaaaa');
		expect(after?.sessionToken).toBe('keep-me');
		expect(after?.role).toBe('gm');
	});

	it('does nothing when touching a room this browser was never in', () => {
		touchSession('aaaaaa');
		expect(listSessions()).toHaveLength(0);
	});

	it('removes a room from the list without touching the others', () => {
		saveSession(session('aaaaaa'));
		saveSession(session('bbbbbb'));

		clearSession('aaaaaa');

		expect(listSessions().map((s) => s.roomSlug)).toEqual(['bbbbbb']);
		expect(loadSession('aaaaaa')).toBeNull();
	});

	// These keys can have been written by an older build, and one bad
	// entry shouldn't cost someone the rest of their rooms.
	it('skips unreadable entries rather than losing the whole list', () => {
		saveSession(session('aaaaaa'));
		window.localStorage.setItem('longtable:session:broken', '{not json');
		window.localStorage.setItem('longtable:session:empty', '{}');

		expect(listSessions().map((s) => s.roomSlug)).toEqual(['aaaaaa']);
	});

	it('ignores unrelated localStorage keys', () => {
		saveSession(session('aaaaaa'));
		window.localStorage.setItem('longtable:theme', 'dark');
		window.localStorage.setItem('something-else', 'value');

		expect(listSessions()).toHaveLength(1);
	});

	// Sessions predating lastOpenedAt still have to appear, just not
	// jump the queue.
	it('keeps a session saved before ordering existed, sorted last', () => {
		window.localStorage.setItem('longtable:session:old111', JSON.stringify(session('old111')));
		saveSession(session('new111'));

		expect(listSessions().map((s) => s.roomSlug)).toEqual(['new111', 'old111']);
	});
});
