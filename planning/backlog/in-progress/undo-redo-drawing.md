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

## Still open: token moves

[room-member-undo-own-token-move](../../user-stories/room-member-undo-own-token-move.md) is
untouched. Its own story defers it ("tracked when we cover the tokens feature area"), and it needs
a different shape of history entry — a position to move back to rather than a drawing to recreate
— though it can reuse the same stack and the same `token.move` command.

## Related user stories

- [room-member-undo-drawing](../../user-stories/room-member-undo-drawing.md)
- [room-member-undo-own-token-move](../../user-stories/room-member-undo-own-token-move.md)
