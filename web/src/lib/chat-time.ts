// When a chat entry landed, in the two forms the log shows it.
//
// Every message has carried `createdAt` since the table was written —
// an RFC3339 string in UTC, minted server-side so a browser with a wrong
// clock doesn't reorder the room's history. Rendering it is this
// module's whole job, and it is a module rather than three lines in the
// panel because "what time is it in this string" has a surprising number
// of ways to come out empty, and the panel is not the place to find that
// out.

/**
 * The time of day, as the log shows it against each entry: `14:32`.
 *
 * The viewer's own clock and locale — a table spread across two
 * timezones each sees the times they were looking at, which is what
 * makes "you said that an hour ago" agree with the wall.
 */
export function timeOfDay(createdAt: string): string {
	const at = parse(createdAt);
	if (!at) return '';
	return `${String(at.getHours()).padStart(2, '0')}:${String(at.getMinutes()).padStart(2, '0')}`;
}

/**
 * The whole thing, for the tooltip: date, time and seconds.
 *
 * The short form above is deliberately ambiguous about which *day* a
 * message is from, because almost every reader is asking about this
 * session. This is where the other question gets answered, and it's why
 * the short form doesn't need to grow a date separator.
 */
export function fullTimestamp(createdAt: string): string {
	const at = parse(createdAt);
	if (!at) return '';
	return at.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'medium' });
}

/**
 * Whether two entries belong to the same day, which is what decides
 * where a date goes in the log.
 *
 * Compared on the reader's own calendar rather than on the ISO strings:
 * two messages either side of midnight UTC are the same evening in
 * Sydney, and the log is read by whoever is looking at it. An unreadable
 * timestamp is never the same day as anything, so a broken entry can't
 * swallow the heading of the day it landed in.
 */
export function sameDay(a: string, b: string): boolean {
	const first = parse(a);
	const second = parse(b);
	if (!first || !second) return false;
	return (
		first.getFullYear() === second.getFullYear() &&
		first.getMonth() === second.getMonth() &&
		first.getDate() === second.getDate()
	);
}

/**
 * The heading above the first entry of a day: `Today`, `Yesterday`, or
 * the date itself.
 *
 * `now` is a parameter so this is testable without pretending to control
 * the clock — the panel passes nothing and gets the real one.
 */
export function dayLabel(createdAt: string, now: Date = new Date()): string {
	const at = parse(createdAt);
	if (!at) return '';

	const yesterday = new Date(now);
	yesterday.setDate(yesterday.getDate() - 1);

	if (sameDayAs(at, now)) return 'Today';
	if (sameDayAs(at, yesterday)) return 'Yesterday';
	return at.toLocaleDateString(undefined, { dateStyle: 'long' });
}

function sameDayAs(a: Date, b: Date): boolean {
	return (
		a.getFullYear() === b.getFullYear() &&
		a.getMonth() === b.getMonth() &&
		a.getDate() === b.getDate()
	);
}

/**
 * A message with no usable timestamp renders no timestamp, rather than
 * `Invalid Date` or `NaN:NaN`.
 *
 * Reachable without anything being broken: an optimistic message minted
 * client-side has whatever the client put on it, and a row written
 * before a column existed is this codebase's recurring shape of bug.
 */
function parse(createdAt: string): Date | null {
	if (!createdAt) return null;
	const at = new Date(createdAt);
	return Number.isNaN(at.getTime()) ? null : at;
}
