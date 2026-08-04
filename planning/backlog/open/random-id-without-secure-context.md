---
title: Mint ids without crypto.randomUUID, so drawing survives a LAN address
created: 2026-08-04
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
section](../../../.claude/skills/longtable-feature/references/ws-protocol.md).

## Related

- [pinch-zoom-touch-devices](pinch-zoom-touch-devices.md) — the other thing in the way of playing
  from a tablet, and found in the same sitting.

## Blocks

[full-bleed-map-layout](full-bleed-map-layout.md) is a deliberately phone-facing redesign, and a
phone at the table reaches the server on a LAN address — which is precisely where this bug lives.
Shipping that layout on top of this would mean drawing and pings being broken for exactly the
clients it was built for.
