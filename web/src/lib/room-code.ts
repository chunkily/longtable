/**
 * Turning whatever someone pasted into a room code.
 *
 * Rooms aren't listed anywhere, so the code is how you get into one — and
 * it arrives by whatever route the group already uses. People paste the
 * whole URL, the path out of it, or read the six characters off someone
 * else's screen and type those. All three are the same intent and none of
 * them is a mistake worth an error message.
 *
 * "Room code" is the word for this everywhere a person can see it — the
 * home page, the join screen, the README, the hosting guide. `slug` is
 * still what the route parameter and the database column are called,
 * because that's a URL shape rather than something anyone is asked to
 * read out.
 */

/**
 * The room-code alphabet, mirroring `slugAlphabet` in
 * `internal/store/slug.go`. It drops `0`, `o`, `1`, `l` and `i` so a code
 * read aloud or copied off a screen can't be ambiguous — which is exactly
 * the situation this function exists to serve, so the two have to agree.
 */
const CODE_PATTERN = /^[abcdefghjkmnpqrstuvwxyz23456789]{6}$/;

/**
 * Extracts a room code from a pasted link, path or bare code, or returns
 * null if there's no plausible code in there.
 *
 * Deliberately lenient about *shape* and strict about the code itself:
 * anything ending in something code-shaped is accepted, whatever precedes
 * it, but the last segment has to actually look like one. That way a typo
 * produces "that doesn't look like a room code" rather than a trip to a
 * room that was never going to exist.
 *
 * Case is folded because codes are lowercase and phones capitalise the
 * first letter of a field without being asked.
 */
export function parseRoomCode(input: string): string | null {
	const trimmed = input.trim().toLowerCase();
	if (!trimmed) return null;

	// Strip a query string or fragment before looking at path segments, so
	// a link copied out of an address bar with tracking junk on the end
	// still resolves.
	const withoutSuffix = trimmed.split(/[?#]/)[0];
	const segments = withoutSuffix.split('/').filter((segment) => segment !== '');
	const candidate = segments[segments.length - 1];

	if (!candidate || !CODE_PATTERN.test(candidate)) return null;
	return candidate;
}
