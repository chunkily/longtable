// Ids for things the client mints itself — a stroke it has already drawn,
// a ping it has already put on the map — so the echo can be matched to
// what's on screen.
//
// `crypto.randomUUID` can't be called directly, which is the whole reason
// this module exists. It's defined only in a **secure context**: HTTPS, or
// a localhost origin. Longtable's deployment story is the opposite — the
// GM runs the binary and everyone else opens `http://192.168.x.x:8080`,
// which will never be a secure context without certificates nobody on a
// home LAN is going to issue. So for every Player it is `undefined`, and
// calling it threw: drawing broke on stroke end and pings broke on
// receipt, while the GM on localhost saw nothing wrong.
//
// `crypto.getRandomValues` is *not* gated on a secure context, so the
// fallback is a real v4 UUID rather than a weaker id.

/**
 * A v4 UUID in the canonical lowercase hyphenated spelling.
 *
 * The spelling is load-bearing, not cosmetic. `isCanonicalUUID` on the Go
 * side rejects the braced and URN forms on purpose, because the id is
 * echoed back and any other spelling would return as a different string
 * from the one this client is holding. A simpler fallback — a counter, a
 * random hex blob — would pass every test on localhost and be refused by
 * the server the moment it ran on a LAN, which is the same asymmetry that
 * hid the original bug.
 */
export function randomId(): string {
	if (typeof crypto.randomUUID === 'function') return crypto.randomUUID();

	const bytes = crypto.getRandomValues(new Uint8Array(16));
	// Version 4 in the high nibble of byte 6, and the RFC 4122 variant in
	// the top bits of byte 8. Without these it's random hex that merely
	// looks like a UUID, and `uuid.Parse` on the Go side would take it —
	// but it would be a lie, and the next thing to validate it properly
	// would reject ids already in the database.
	bytes[6] = (bytes[6] & 0x0f) | 0x40;
	bytes[8] = (bytes[8] & 0x3f) | 0x80;

	const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
	return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
