---
title: No drawing on right mouse button
created: 2026-07-29
status: done
tags: [drawing, bug]
story: room-member-no-draw-on-right-click
---

Drawing should not occur when user uses the right mouse button.

Widened during the work to cover every tool that opens a gesture on a press, not the drawing
tools alone — right-clicking with the eraser active still erased, which is the same accident
with the same cause. See the note on scope in the linked story.

## What shipped

The right mouse button no longer does anything on the map. Not drawing, not erasing, not
revealing fog, not pinging, not measuring. The left button and touch are untouched.

One helper, `isPrimaryPointer` in `game-canvas.svelte`, checked by every handler that opens a
gesture. Three things about it are load-bearing:

- **It tests "is a mouse event *and* not the primary button", not "button !== 0".** `TouchEvent`
  carries no `button` property at all, so the obvious spelling reads `undefined !== 0`, rejects
  every touch, and silently breaks every tool on a tablet — where there is no right button to
  guard against in the first place.
- **`mouseup` is guarded as well as `mousedown`.** Blocking only the press stops a right-drag,
  but a stray right-click *during* a left-button gesture still fires `mouseup`, which would
  commit the drawing or the fog reveal early, at whatever state it had reached.
- **`mouseleave` is deliberately *not* guarded**, and for measure it was split out of a combined
  `'mouseup.tool touchend.tool mouseleave.tool'` handler to keep it that way. Leaving the canvas
  has to end the gesture whatever the buttons are doing, because no `mouseup` is coming once the
  pointer is released outside — that is what stops a measurement freezing at the edge of everyone
  else's map. Folding it back in would happen to work, since `mouseleave` reports `button === 0`,
  but only by accident.

Two traps found while testing this, both of which produced a green test that asserted nothing
until a left-button positive control was added alongside each right-button assertion:

- **The freehand tool paints a cursor ring on the preview layer**, showing how wide the line will
  be, and it correctly follows the pointer whatever the buttons are doing. "The preview layer is
  empty" is therefore false for freehand even when nothing is being drawn. The test walks the
  same path with no button held first and asserts the right-drag adds nothing beyond the ring.
- **Counting non-transparent pixels cannot see a GM's fog reveal.** See `references/testing.md`.

## What the free button became

The right mouse button is no longer inert: as of 2026-08-14 it **drags the map**, in every tool
— see [right-click-pan](right-click-pan.md), which is the "other purposes" the linked story left
room for. `isPrimaryPointer` is unchanged and still guards every tool handler, which is what keeps
the rule above true: the pan is bound separately, in its own `.pan` namespace, and a right press
mid-stroke pans the map *without* the tool noticing — that is how a ruler gets dragged past the
edge of the screen. One consequence worth knowing: the browser's context menu is now suppressed
across the whole stage, so a right-click on the map has exactly one meaning.
