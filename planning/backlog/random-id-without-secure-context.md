---
title: Mint ids without crypto.randomUUID, so drawing survives a LAN address
created: 2026-08-04
status: done
tags: [drawing, mobile, defect]
---

`crypto.randomUUID()` is only defined in a **secure context** — HTTPS, or a `localhost` /
`127.0.0.1` origin. Longtable's entire deployment story is the opposite of that: the GM runs the
binary and everyone else opens `http://192.168.x.x:8080`, which is not a secure context and won't
become one without certificates nobody on a home LAN is going to issue.

So `crypto.randomUUID` is `undefined` for every player, and two things throw. Both call sites are
in `web/src/lib/room.svelte.ts`:

- **`createDrawing`** (~line 541) mints the stroke's id client-side so the echo can be matched to
  what's already on screen. It throws the moment a stroke is finished, so a player simply cannot
  draw.
- **The `ping` case in `handleEnvelope`** (~line 1014) mints a local id for an incoming ping. This
  one throws on *receipt*, so a ping sent by anyone — including the GM — breaks while being folded
  in, and never appears.

The GM is fine and everyone else is not, which is exactly why this has never shown up: the e2e
suite drives `localhost`, and so does every developer. Found while setting up to view a room from
a tablet on the LAN, before the tablet was even opened.

## Work

- [ ] `randomId()` in its own module under `web/src/lib/`, preferring `crypto.randomUUID` and
      falling back to `crypto.getRandomValues` — which *is* available in insecure contexts, so
      the fallback is a real v4 UUID rather than a weaker id
- [ ] A vitest that stubs `crypto.randomUUID` away and asserts the fallback still matches the
      canonical form. That's the only way this gets covered; no browser test runs in an insecure
      context today
- [ ] Both call sites in `room.svelte.ts` use it
- [ ] Grep for other secure-context-only APIs before calling it done. `navigator.clipboard` is
      the likely next one — it isn't used yet, and a "copy the join link" button is an obvious
      future feature that would hit the same wall

## The constraint that decides the fallback

**It has to produce the canonical lowercase hyphenated UUID spelling.** A client-supplied drawing
id is checked with `isCanonicalUUID` in `internal/ws/hub.go`, which rejects the braced and URN
spellings on purpose: the id is echoed back, and any other spelling would return as a different
string from the one the client is holding. `token.create` takes an optional client-chosen id
through the same check.

So a fallback along the lines of `` `id-${counter}` `` would pass every test on localhost and be
refused by the server the moment it ran on a LAN — the same asymmetry that hid the original bug,
reintroduced by the fix. See [ws-protocol.md's "Ids"
section](../../.claude/skills/longtable-feature/references/ws-protocol.md).

## Related

- [pinch-zoom-touch-devices](pinch-zoom-touch-devices.md) — the other thing in the way of
  playing from a tablet, and found in the same sitting. Still open.

## Blocks

[full-bleed-map-layout](full-bleed-map-layout.md) is a deliberately phone-facing redesign,
and a phone at the table reaches the server on a LAN address — which is precisely where this bug
lives. Shipping that layout on top of this would mean drawing and pings being broken for exactly
the clients it was built for. **No longer blocking as of this item shipping**; pinch-zoom still is.

## What shipped

`randomId()` in `web/src/lib/random-id.ts`, used by both call sites. It prefers
`crypto.randomUUID` and falls back to a v4 UUID assembled from `crypto.getRandomValues`, which
isn't gated on a secure context. Nothing changed on the Go side — the fallback's whole job is to
produce ids the existing `isCanonicalUUID` already accepts.

Three things worth knowing before touching this area again.

**Never call `crypto.randomUUID` directly.** That's now written into `CLAUDE.md` and the `Ids`
section of `ws-protocol.md`, because the failure mode is so asymmetric: it works perfectly for
whoever is developing it and for the GM, and fails for every Player. The next API with this shape
is `navigator.clipboard` — unused today, and a "copy the join link" button would walk straight into
it.

**The fallback sets the version and variant nibbles even though the server doesn't check them.**
`isCanonicalUUID` is `uuid.Parse` plus a round-trip comparison, so random hex in the right shape
would be accepted. Setting them anyway means the ids aren't a lie, and the next thing to validate
a UUID properly won't reject rows already in the database.

**The e2e spec is the part that actually proves it.** The vitest beside the module checks the
fallback's spelling; only `web/e2e/insecure-context.spec.ts` checks that the *server* accepts what
it produces, which is where a plausible-looking wrong fallback would have failed. It fakes the
insecure context with `page.addInitScript`, since Playwright drives localhost and can't reach a
real one. Both of its tests were confirmed to fail against the unfixed code before being kept — a
test for a bug that passes on the broken version is worth nothing, and this one is easy to write
that way by accident.

One thing found in passing and deliberately not fixed here: `asset-library.spec.ts:184` is flaky
under the full parallel run, failing roughly one run in three and passing in isolation. Confirmed
pre-existing by running the suite repeatedly at HEAD with no local changes, so it is unrelated to
this work.

**Fixed on 2026-08-05** — see [e2e-flakes](e2e-flakes.md). The "passing in isolation" reading
above was the misleading part: it made this look like a load or timing problem, when the test was
actually asserting behaviour the code has never had. It passed whenever it managed to check
before the page refreshed.
