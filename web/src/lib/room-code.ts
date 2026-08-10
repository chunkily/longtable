/**
 * Validating a room code someone has typed in.
 *
 * Rooms aren't listed anywhere, so the code is how you get into one. It
 * arrives by whatever route the group already uses — read off someone
 * else's screen, or in a message — and this turns what was typed into
 * either a code or nothing.
 *
 * **Codes only. A link is not accepted, and that is deliberate.** An
 * earlier version pulled the code out of the last path segment, so a
 * whole pasted URL worked too. That was dropped once the join box became
 * a six-character field that says so: a link is already a link, and
 * clicking it lands you in the room without this box being involved at
 * all. The case the leniency actually served — someone who has a link,
 * on the machine they want to play on, choosing to paste it into a text
 * field instead of following it — is rare enough not to pay for with a
 * parser that accepts things the field doesn't advertise.
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
 * Returns the room code in `input`, or null if it isn't one.
 *
 * Surrounding whitespace is forgiven because it rides along with
 * anything pasted, and case is folded because codes are lowercase and
 * phones capitalise the first letter of a field without being asked.
 * Nothing else is: six characters from the alphabet above, or null.
 */
export function parseRoomCode(input: string): string | null {
	const candidate = input.trim().toLowerCase();
	return CODE_PATTERN.test(candidate) ? candidate : null;
}
