import { describe, expect, it } from 'vitest';
import { parseRoomCode } from './room-code';

describe('parseRoomCode', () => {
	it('takes six characters from the code alphabet', () => {
		expect(parseRoomCode('7wdbtb')).toBe('7wdbtb');
	});

	it('ignores surrounding whitespace, which rides along with a paste', () => {
		expect(parseRoomCode('  7wdbtb  ')).toBe('7wdbtb');
		expect(parseRoomCode('\n7wdbtb\n')).toBe('7wdbtb');
	});

	// Phones capitalise the first character of a text field unprompted,
	// and room codes are lowercase.
	it('folds case, since a phone will capitalise what someone types', () => {
		expect(parseRoomCode('7WDBTB')).toBe('7wdbtb');
		expect(parseRoomCode('7Wdbtb')).toBe('7wdbtb');
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
		expect(parseRoomCode('what is the room again')).toBeNull();
	});

	// A link used to be accepted, by taking the last path segment. It
	// isn't any more, and this is the test that says so on purpose rather
	// than by omission — a link is already a link, and following it lands
	// you in the room without this field being involved.
	it('rejects a link or a path, even one with a real code in it', () => {
		expect(parseRoomCode('http://192.168.1.5:8080/r/7wdbtb')).toBeNull();
		expect(parseRoomCode('/r/7wdbtb')).toBeNull();
		expect(parseRoomCode('http://host:8080/r/7wdbtb?from=discord')).toBeNull();
		expect(parseRoomCode('7wdbtb/')).toBeNull();
	});
});
