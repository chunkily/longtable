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

## Related user stories

- [gm-toggle-token-owner-only-movement](../../user-stories/gm-toggle-token-owner-only-movement.md)
- [player-move-owned-token-when-locked](../../user-stories/player-move-owned-token-when-locked.md)
