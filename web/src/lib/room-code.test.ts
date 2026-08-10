import { describe, expect, it } from 'vitest';
import { parseRoomCode } from './room-code';

describe('parseRoomCode', () => {
	// The three ways a room code actually reaches someone: the whole URL
	// out of an address bar, the path out of a chat message, and six
	// characters read off a screen.
	it('takes a full link, a path, or a bare code', () => {
		expect(parseRoomCode('http://192.168.1.5:8080/r/7wdbtb')).toBe('7wdbtb');
		expect(parseRoomCode('/r/7wdbtb')).toBe('7wdbtb');
		expect(parseRoomCode('7wdbtb')).toBe('7wdbtb');
	});

	it('ignores surrounding whitespace and a trailing slash', () => {
		expect(parseRoomCode('  http://host:8080/r/7wdbtb/  ')).toBe('7wdbtb');
		expect(parseRoomCode('\n7wdbtb\n')).toBe('7wdbtb');
	});

	// A link copied out of an address bar can carry a fragment or query
	// that isn't part of the code.
	it('drops a query string or fragment', () => {
		expect(parseRoomCode('http://host:8080/r/7wdbtb?from=discord')).toBe('7wdbtb');
		expect(parseRoomCode('http://host:8080/r/7wdbtb#chat')).toBe('7wdbtb');
	});

	// Phones capitalise the first character of a text field unprompted,
	// and room codes are lowercase.
	it('folds case, since a phone will capitalise what someone types', () => {
		expect(parseRoomCode('7WDBTB')).toBe('7wdbtb');
		expect(parseRoomCode('http://host:8080/r/7Wdbtb')).toBe('7wdbtb');
	});

	// The alphabet drops 0/o/1/l/i precisely so a code read aloud isn't
	// ambiguous, so anything containing them was mistyped.
	it('rejects characters the room-code alphabet deliberately excludes', () => {
		for (const bad of ['7wdbt0', '7wdbto', '7wdbt1', '7wdbtl', '7wdbti']) {
			expect(parseRoomCode(bad)).toBeNull();
		}
	});

	it('rejects anything that is not six characters', () => {
		expect(parseRoomCode('7wdbt')).toBeNull();
		expect(parseRoomCode('7wdbtbb')).toBeNull();
	});

	it('rejects empty and obviously non-code input', () => {
		expect(parseRoomCode('')).toBeNull();
		expect(parseRoomCode('   ')).toBeNull();
		expect(parseRoomCode('https://example.com')).toBeNull();
		expect(parseRoomCode('what is the room again')).toBeNull();
	});

	// Being strict about the last segment is what turns a typo into "that
	// doesn't look like a room code" rather than a trip to a room that was
	// never going to exist.
	it('rejects a well-formed link whose code is malformed', () => {
		expect(parseRoomCode('http://host:8080/r/not-a-code')).toBeNull();
	});
});
