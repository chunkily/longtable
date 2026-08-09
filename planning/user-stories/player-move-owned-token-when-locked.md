---
title: Player moves only owned tokens when locked
created: 2026-07-29
status: done
---

As a Player
I want to be able to move tokens I own even when the room's owner-only movement setting is on
So that I can still control my own character while other tokens are protected from me

## Acceptance criteria

- [ ] When owner-only movement is on, I can move a token if and only if I'm its owner
- [ ] Attempting to move a token I don't own is rejected with no effect when the setting is on
- [ ] This restriction doesn't apply when owner-only movement is off (see gm-toggle-token-owner-only-movement)

## Verified 2026-08-09

All three hold. The second reads literally — "rejected with no effect" — and there is a test
asserting the token is still on the square it started from after a refused move, not merely that
an error came back.

Worth knowing: a locked token doesn't just fail to move, it doesn't pick up at all, and it
deliberately swallows the press rather than letting the map pan under it. See the "What shipped"
note on [token-move-ownership-lock](../backlog/token-move-ownership-lock.md) — the honest version
of "rejected with no effect" turned out to be a canvas problem, not a protocol one.
