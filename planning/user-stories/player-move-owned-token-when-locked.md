---
title: Player moves only owned tokens when locked
created: 2026-07-29
status: incomplete
---

As a Player
I want to be able to move tokens I own even when the room's owner-only movement setting is on
So that I can still control my own character while other tokens are protected from me

## Acceptance criteria

- [ ] When owner-only movement is on, I can move a token if and only if I'm its owner
- [ ] Attempting to move a token I don't own is rejected with no effect when the setting is on
- [ ] This restriction doesn't apply when owner-only movement is off (see gm-toggle-token-owner-only-movement)
