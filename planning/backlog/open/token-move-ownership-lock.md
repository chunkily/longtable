---
title: Token move ownership lock
created: 2026-07-29
tags: [tokens]
---

Let a GM restrict token movement to owners only, off by default (matches current open behavior
where any Room Member can move any token). GM can always move any token regardless of the
setting.

**No longer blocked on owners existing.**
[token-size-and-owner-pickers](../done/token-size-and-owner-pickers.md) has shipped the picker on
both token dialogs, `token.update` carries the owner, and `requireOwnerInRoom` already guarantees
an owner is someone in this room — so a permission check here can trust
`token.OwnerParticipantID` without re-validating it. What's missing is the per-room setting
itself and the check in `handleTokenMove`, which is currently open to any Room Member.

One thing this doesn't have to think about: undoing a token move
([undo-redo-drawing](../done/undo-redo-drawing.md)) sends an ordinary `token.move`, so whatever
check lands in `handleTokenMove` governs the undo too, with nothing extra to write. That's
deliberate, and it's what
[room-member-undo-own-token-move](../../user-stories/room-member-undo-own-token-move.md)'s third
criterion asks for — undo obeying the move rules in force rather than routing around them.

Two things from the 2026-08-04 design session. The toggle's home is `Manage room`, the third entry
in the side panel's menu — see [full-bleed-map-layout](full-bleed-map-layout.md). And
[player-created-tokens](player-created-tokens.md) is what finally makes this feature mean
something: a Player-made token is owned by its creator without anyone choosing an owner, so a table
that turns the lock on gets sensible behaviour by default rather than needing the GM to assign
every token first.

## Related user stories

- [gm-toggle-token-owner-only-movement](../../user-stories/gm-toggle-token-owner-only-movement.md)
- [player-move-owned-token-when-locked](../../user-stories/player-move-owned-token-when-locked.md)
