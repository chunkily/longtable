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

## Related user stories

- [room-member-undo-drawing](../../user-stories/room-member-undo-drawing.md)
- [room-member-undo-own-token-move](../../user-stories/room-member-undo-own-token-move.md)
