import { describe, expect, it } from 'vitest';
import { parseInvite } from './invite';

describe('parseInvite', () => {
	// The three ways an invite actually reaches someone: the whole URL out
	// of an address bar, the path out of a chat message, and six
	// characters read off a screen.
	it('takes a full link, a path, or a bare code', () => {
		expect(parseInvite('http://192.168.1.5:8080/r/7wdbtb')).toBe('7wdbtb');
		expect(parseInvite('/r/7wdbtb')).toBe('7wdbtb');
		expect(parseInvite('7wdbtb')).toBe('7wdbtb');
	});

	it('ignores surrounding whitespace and a trailing slash', () => {
		expect(parseInvite('  http://host:8080/r/7wdbtb/  ')).toBe('7wdbtb');
		expect(parseInvite('\n7wdbtb\n')).toBe('7wdbtb');
	});

	// A link copied out of an address bar can carry a fragment or query
	// that isn't part of the slug.
	it('drops a query string or fragment', () => {
		expect(parseInvite('http://host:8080/r/7wdbtb?from=discord')).toBe('7wdbtb');
		expect(parseInvite('http://host:8080/r/7wdbtb#chat')).toBe('7wdbtb');
	});

	// Phones capitalise the first character of a text field unprompted,
	// and slugs are lowercase.
	it('folds case, since a phone will capitalise what someone types', () => {
		expect(parseInvite('7WDBTB')).toBe('7wdbtb');
		expect(parseInvite('http://host:8080/r/7Wdbtb')).toBe('7wdbtb');
	});

	// The slug alphabet drops 0/o/1/l/i precisely so a code read aloud
	// isn't ambiguous, so anything containing them was mistyped.
	it('rejects characters the slug alphabet deliberately excludes', () => {
		for (const bad of ['7wdbt0', '7wdbto', '7wdbt1', '7wdbtl', '7wdbti']) {
			expect(parseInvite(bad)).toBeNull();
		}
	});

	it('rejects anything that is not six characters', () => {
		expect(parseInvite('7wdbt')).toBeNull();
		expect(parseInvite('7wdbtbb')).toBeNull();
	});

	it('rejects empty and obviously non-invite input', () => {
		expect(parseInvite('')).toBeNull();
		expect(parseInvite('   ')).toBeNull();
		expect(parseInvite('https://example.com')).toBeNull();
		expect(parseInvite('what is the room again')).toBeNull();
	});

	// Being strict about the last segment is what turns a typo into "that
	// doesn't look like an invite" rather than a trip to a room that was
	// never going to exist.
	it('rejects a well-formed link whose slug is malformed', () => {
		expect(parseInvite('http://host:8080/r/not-a-slug')).toBeNull();
	});
});
