import { afterEach, describe, expect, it, vi } from 'vitest';
import { randomId } from './random-id';

// The canonical lowercase hyphenated v4 spelling, which is the only one
// `isCanonicalUUID` on the Go side accepts. The `4` and the `[89ab]` are
// the version and variant nibbles, not decoration.
const CANONICAL_V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

// What `crypto` looks like outside a secure context: `getRandomValues` is
// there, `randomUUID` simply isn't. This is every Player on
// `http://192.168.x.x:8080`, and the case no browser test can reach —
// Playwright drives localhost, which is always a secure context.
function withoutRandomUUID() {
	const getRandomValues = crypto.getRandomValues.bind(crypto);
	vi.stubGlobal('crypto', { getRandomValues });
}

describe('randomId', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('mints a canonical v4 uuid', () => {
		expect(randomId()).toMatch(CANONICAL_V4);
	});

	// Asserted with a stub rather than inferred from the shape, because
	// both paths produce the same shape — so without this the GM's branch
	// would look covered while only the fallback was ever running.
	it('uses the platform uuid where there is one', () => {
		const randomUUID = vi.fn(() => '2f1c8b40-3d5e-4a91-9c07-5b6d2e8f4a13');
		vi.stubGlobal('crypto', { randomUUID, getRandomValues: crypto.getRandomValues.bind(crypto) });

		expect(randomId()).toBe('2f1c8b40-3d5e-4a91-9c07-5b6d2e8f4a13');
		expect(randomUUID).toHaveBeenCalledOnce();
	});

	// The bug this module exists for: calling `crypto.randomUUID` where it
	// is undefined threw, so a Player couldn't finish a stroke and nobody's
	// pings survived being folded in.
	it('still mints one where crypto.randomUUID does not exist', () => {
		withoutRandomUUID();

		expect(randomId()).toMatch(CANONICAL_V4);
	});

	// A fallback that returned `id-1` would pass on localhost, where it
	// never runs, and be refused by the server on the LAN, where it always
	// does. Asserting the shape is what stops that being reintroduced.
	it('sets the version and variant bits in the fallback, not just random hex', () => {
		withoutRandomUUID();

		for (let i = 0; i < 50; i++) {
			const id = randomId();
			expect(id[14]).toBe('4');
			expect('89ab').toContain(id[19]);
		}
	});

	it('does not repeat itself', () => {
		withoutRandomUUID();

		const ids = new Set(Array.from({ length: 500 }, () => randomId()));
		expect(ids.size).toBe(500);
	});
});
