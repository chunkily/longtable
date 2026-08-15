import { describe, expect, it } from 'vitest';
import { fullTimestamp, timeOfDay } from './chat-time';

// Built from local components and read back as an ISO string, so these
// assert the round trip rather than the machine's timezone: the server
// sends UTC, the panel shows the reader's own clock, and a test that
// hardcoded either would pass in one country.
function isoAt(hours: number, minutes: number, seconds = 0): string {
	return new Date(2026, 7, 15, hours, minutes, seconds).toISOString();
}

describe('timeOfDay', () => {
	it('shows the time the message landed, on the reader clock', () => {
		expect(timeOfDay(isoAt(14, 32))).toBe('14:32');
	});

	// A quarter past nine has to be 09:15, not 9:15: the log is a column
	// of times and a ragged one is harder to scan than a padded one.
	it('pads to a fixed width', () => {
		expect(timeOfDay(isoAt(9, 5))).toBe('09:05');
	});

	it('is 24-hour, so midnight and midday are distinguishable', () => {
		expect(timeOfDay(isoAt(0, 0))).toBe('00:00');
		expect(timeOfDay(isoAt(12, 0))).toBe('12:00');
		expect(timeOfDay(isoAt(23, 59))).toBe('23:59');
	});

	// Renders nothing rather than "Invalid Date" or "NaN:NaN". An
	// optimistic message carries whatever the client put on it.
	it('says nothing about a timestamp it cannot read', () => {
		expect(timeOfDay('')).toBe('');
		expect(timeOfDay('not a date')).toBe('');
	});
});

describe('fullTimestamp', () => {
	// Loosely asserted on purpose: the exact wording is the runtime's
	// locale data, which is not this module's promise. What is promised
	// is that the tooltip answers the question the short form doesn't —
	// which day — and gets down to seconds.
	it('carries the date and the seconds the short form leaves out', () => {
		const full = fullTimestamp(isoAt(14, 32, 7));
		expect(full).toContain('2026');
		expect(full).toMatch(/\b07\b/);
		expect(full.length).toBeGreaterThan(timeOfDay(isoAt(14, 32, 7)).length);
	});

	it('says nothing about a timestamp it cannot read', () => {
		expect(fullTimestamp('nonsense')).toBe('');
	});
});
