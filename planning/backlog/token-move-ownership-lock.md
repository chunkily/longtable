---
title: Token move ownership lock
created: 2026-07-29
status: open
tags: [tokens]
---

Let a GM restrict token movement to owners only, off by default (matches current open behavior
where any Room Member can move any token). GM can always move any token regardless of the
setting.

**No longer blocked on owners existing.**
[token-size-and-owner-pickers](token-size-and-owner-pickers.md) has shipped the picker on
both token dialogs, `token.update` carries the owner, and `requireOwnerInRoom` already guarantees
an owner is someone in this room — so a permission check here can trust
`token.OwnerParticipantID` without re-validating it. What's missing is the per-room setting
itself and the check in `handleTokenMove`, which is currently open to any Room Member.

**There is now a worked example of an ownership check to copy.**
[token-hp-condition-tracker](token-hp-condition-tracker.md) has shipped the first rule in
the codebase where owning a token means anything: `handleTokenUpdate`'s role gate is per field, and
a Player who owns a token may change its trackers and conditions. Read that handler before writing
this one — in particular the part about a **hidden** token being refused to a non-GM in the exact
words of a token that doesn't exist, *even to its own owner*, since the same reasoning applies to a
move and getting it wrong is a quiet information leak rather than a visible bug.

One thing this doesn't have to think about: undoing a token move
([undo-redo-drawing](undo-redo-drawing.md)) sends an ordinary `token.move`, so whatever
check lands in `handleTokenMove` governs the undo too, with nothing extra to write. That's
deliberate, and it's what
[room-member-undo-own-token-move](../user-stories/room-member-undo-own-token-move.md)'s third
criterion asks for — undo obeying the move rules in force rather than routing around them.

Two things from the 2026-08-04 design session. The toggle's home is `Manage room`, the third entry
in the side panel's menu — see [full-bleed-map-layout](full-bleed-map-layout.md). And
[player-created-tokens](player-created-tokens.md) is what finally makes this feature mean
something: a Player-made token is owned by its creator without anyone choosing an owner, so a table
that turns the lock on gets sensible behaviour by default rather than needing the GM to assign
every token first.

**Its home now exists.** [full-bleed-map-layout](full-bleed-map-layout.md) shipped a
`Manage room` dialog (`web/src/lib/components/manage-room-dialog.svelte`), opened from the room
menu and GM-only, holding nothing yet and saying so. This is one of the settings it is waiting
for: add it there rather than inventing a second place for room settings to live, and delete the
"nothing to configure yet" paragraph once something is.

## Related user stories

- [gm-toggle-token-owner-only-movement](../user-stories/gm-toggle-token-owner-only-movement.md)
- [player-move-owned-token-when-locked](../user-stories/player-move-owned-token-when-locked.md)
