import { describe, expect, it } from 'vitest';
import { dayLabel, fullTimestamp, sameDay, timeOfDay } from './chat-time';

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

describe('sameDay', () => {
	it('groups two entries from the same day', () => {
		expect(sameDay(isoAt(9, 15), isoAt(23, 58))).toBe(true);
	});

	// The pair the log is really about: last thing said last night, first
	// thing said this morning. Nothing about the clock times says they are
	// different days, which is the whole reason a date goes between them.
	it('separates entries either side of midnight', () => {
		const lastNight = new Date(2026, 7, 15, 23, 58).toISOString();
		const thisMorning = new Date(2026, 7, 16, 9, 12).toISOString();
		expect(sameDay(lastNight, thisMorning)).toBe(false);
	});

	it('separates the same date a year apart', () => {
		const then = new Date(2025, 7, 15, 12, 0).toISOString();
		expect(sameDay(then, isoAt(12, 0))).toBe(false);
	});

	// Never the same day as anything, so a broken entry can't swallow the
	// heading of the day it landed in.
	it('treats an unreadable timestamp as its own day', () => {
		expect(sameDay('nonsense', isoAt(12, 0))).toBe(false);
		expect(sameDay('nonsense', 'nonsense')).toBe(false);
	});
});

describe('dayLabel', () => {
	const now = new Date(2026, 7, 15, 18, 0);

	it('says Today rather than making the reader work out the date', () => {
		expect(dayLabel(isoAt(9, 15), now)).toBe('Today');
	});

	it('says Yesterday for the day before', () => {
		const yesterday = new Date(2026, 7, 14, 21, 30).toISOString();
		expect(dayLabel(yesterday, now)).toBe('Yesterday');
	});

	// Crossing a month boundary backwards, where "the day before" is not
	// "one less than the date".
	it('says Yesterday on the first of the month too', () => {
		const endOfLastMonth = new Date(2026, 6, 31, 20, 0).toISOString();
		expect(dayLabel(endOfLastMonth, new Date(2026, 7, 1, 10, 0))).toBe('Yesterday');
	});

	it('gives the date for anything older', () => {
		const label = dayLabel(new Date(2026, 7, 9, 20, 0).toISOString(), now);
		expect(label).not.toBe('Today');
		expect(label).not.toBe('Yesterday');
		expect(label).toContain('2026');
	});

	it('says nothing about a timestamp it cannot read', () => {
		expect(dayLabel('nonsense', now)).toBe('');
	});
});
