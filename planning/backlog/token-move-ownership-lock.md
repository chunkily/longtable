---
title: Token move ownership lock
created: 2026-07-29
status: done
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

**The premise it was waiting on has shipped.**
[player-created-tokens](player-created-tokens.md) is `status: done`: a Player's token is owned by
its creator with nobody choosing an owner, so a table that turns this lock on gets sensible
behaviour immediately rather than needing the GM to assign every token first. Ownership now also
decides who may *delete* a token (`handleTokenDelete`), which is a second worked example of the
check to copy — and a closer one than the tracker rule, since it is a whole-command gate rather
than a per-field one, which is what a move needs.

**Its home now exists.** [full-bleed-map-layout](full-bleed-map-layout.md) shipped a
`Manage room` dialog (`web/src/lib/components/manage-room-dialog.svelte`), opened from the room
menu and GM-only, holding nothing yet and saying so. This is one of the settings it is waiting
for: add it there rather than inventing a second place for room settings to live, and delete the
"nothing to configure yet" paragraph once something is.

## Related user stories

- [gm-toggle-token-owner-only-movement](../user-stories/gm-toggle-token-owner-only-movement.md)
- [player-move-owned-token-when-locked](../user-stories/player-move-owned-token-when-locked.md)

## What shipped

`room.owner_only_movement`, off by default in the column rather than in Go — a row written by any
path, including one that doesn't exist yet, is then open rather than accidentally locked, which is
the direction that fails safely. `room.setOwnerOnlyMovement` (GM-only) flips it and broadcasts
`room.updated` carrying the **whole room** via a new shared `roomPayload`, the same map
`state.sync` opens with. The next setting to land in `Manage room` needs no new event, and — like
`participantPayload` — it is built field by field so the password hash can't ride along.

`mayMoveToken` is the check, and it is ordered to cost nothing on an ordinary table: a GM returns
immediately with nothing loaded, an unlocked room after one cheap room read, and only a locked
room's non-GM move pays for the token. A hidden token is refused as a missing one — the same
sentence `token.update` and `token.delete` use — but **only when the lock is on**: an open room
has never checked visibility on a move, and quietly making it do so would be a second feature
nobody asked for.

**The undo came free, exactly as this item predicted.** Undoing a move is an ordinary
`token.move`, so the check governs it with nothing extra written; there's a test asserting it,
because "free" is the kind of claim that stops being true silently.

**The part that wasn't free was the canvas, and it is the thing to know before touching this
again.** Making a locked token `draggable: false` is not enough: Konva starts the *stage* drag
from whatever `pointerdown` bubbles up to it, so the first working version panned the entire scene
every time a Player grabbed somebody else's token. It looked like the app misbehaving rather than
like a refusal. A locked token now swallows the press (`e.cancelBubble = true` on
`mousedown.lock`/`touchstart.lock`); `click` is a separate event and still bubbles, so a locked
token can be selected and inspected as before. The e2e caught this rather than the unit tests, and
it could only have been caught by driving a real drag.

The client-side rule lives in one place, `RoomClient.canMoveToken`, so the canvas and anything
asking later can't disagree with each other — and `room.ownerOnlyMovement` is tracked by the token
effect, because a GM flipping the setting has to take hold of everyone's tokens *now* rather than
at their next reload. That's the case a reload-only implementation would have passed every test
except the one that watches two browsers.

The setting is two labelled buttons rather than a switch: there is no switch component in this
project, and "on"/"off" are poor names for a rule about other people's tokens. `Anyone moves
anything` / `Only the owner` says what each state means.
