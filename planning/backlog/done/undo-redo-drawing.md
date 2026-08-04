---
title: Undo for drawing and moves
created: 2026-07-29
tags: [drawing, tokens]
---

Allow for users to undo their moves and drawing edits.

One thing already decided: the eraser can be dragged across several drawings in a single gesture,
and each of those deletions counts as its own undoable action rather than the sweep counting as
one. Undoing a sweep therefore takes as many undos as it erased, which keeps this feature to a
flat per-action history with no grouping to model.

## Done: drawings

Undo and redo for drawing and erasing, via Ctrl+Z / Ctrl+Shift+Z (Ctrl+Y too) and a pair of
toolbar buttons. The history is per session and holds only this client's own actions, which is
what keeps undo from reaching someone else's work on a shared map. It needs no server support:
undoing a drawing is a `draw.delete` and undoing an erase is a `draw.create` under the same id,
both of which already exist, already enforce permission, and already render optimistically.

Two behaviours worth knowing before extending this:

- An entry whose drawing is no longer in a state it can act on — a GM erased the stroke you were
  about to undo — is skipped, and undo moves on to the next thing you did rather than failing.
- Restoring an erased drawing re-creates it under whoever pressed undo, because the server takes
  authorship from the connection and deliberately ignores anything claimed in the payload. So a
  GM who erases a Player's stroke and undoes gets it back under their own name, and the Player can
  no longer erase it themselves. Fixing that properly needs a server-side restore that preserves
  the original author, which is a new command rather than a tweak.

## Done: token deletion

[delete-token](../done/delete-token.md) has **shipped**, and with it the generalisation this item
had been assuming. The history entry is no longer drawing-shaped: `DrawingAction` is now a
discriminated union, `HistoryAction`, and `reverse`/`apply` switch on `kind`. Adding a third
variant is now a case in each of those two switches plus a pair of send helpers — most of the
cost was in the first one.

## Done: token moves

A drag can now be taken back with the same Ctrl+Z and the same toolbar button as everything else,
by anyone at the table — a Player's own moves land on their own stack. The entry is
`{ kind: 'moveToken', tokenId, from, to }`, and it is indeed the one variant holding less than the
whole object; carrying `to` as well as `from` turned out to matter, see below.

## What shipped

Undo and redo now cover drawing, erasing, token deletion and token moves. Nothing was needed on
the Go side: undoing a move is a `token.move` back, which already exists and already broadcasts,
so the token slides home on everyone's screen the same way any other move does.

Three things worth knowing before touching this again.

**The entry carries where the token went, not only where it came from, and that's what keeps undo
to your own moves.** The history has no way to ask who dragged a token last — the hub's
`token.moved` carries no sender. The position stands in for it: `sendMoveToken` refuses unless the
token is still standing where this session's move left it, and `step()` then passes over the entry
and undoes the next thing instead. So a GM who moves a goblin, watches a Player move it again, and
presses Ctrl+Z does *not* yank it back out from under them. This is the same skip that already
covered a stroke someone else erased, and it's what
[room-member-undo-own-token-move](../../user-stories/room-member-undo-own-token-move.md)'s second
criterion asks for.

**`pendingMoves` exists because state lags a move by a round trip.** Nothing about a move is
applied optimistically — deliberately, see the comment on `token.moved` about rebuilding token
groups under a live pointer — so for one round trip a token just dragged is still recorded on the
square it left. Without somewhere to hold the in-flight destination, a Ctrl+Z pressed inside that
window would decide the token had never moved, skip the entry and undo whatever came before it,
and two quick drags in a row would both record the same origin. The map is cleared on
`token.deleted` (no broadcast is coming for a token that's gone) and matched on *coordinates* on
`token.moved`, not just on the id — an id match alone would retire our entry against someone
else's move arriving first.

**A drop back on the square it started from records nothing.** The command still goes out, but an
entry with `from === to` would swallow a whole press of Ctrl+Z doing nothing visible.

The two-browser spec is `web/e2e/token-move-undo.spec.ts`, and it cost two rounds of debugging
worth knowing about: a GM's canvas and a Player's are at different page offsets (different toolbar
heights), and a token still mid-slide can't be grabbed. Both are written up in the skill's
`testing.md`.

## Related user stories

- [room-member-undo-drawing](../../user-stories/room-member-undo-drawing.md)
- [room-member-undo-own-token-move](../../user-stories/room-member-undo-own-token-move.md)
- [gm-delete-token](../../user-stories/gm-delete-token.md)
