---
title: Pinch to zoom the map on a touch device
created: 2026-08-04
tags: [canvas, mobile, ui]
story: room-member-pinch-zoom-map
---

`handleWheel` (`web/src/lib/components/game-canvas.svelte`, ~line 265) is the only thing that ever
changes `stage.scale()`, and a tablet has no wheel. So on a touch device the map is stuck at 1:1
forever: a Player on an iPad can see roughly nine squares of a battle map and has no way to pull
back, which makes the whole thing unusable on the device people are most likely to bring to a
table.

What already works on touch, and shouldn't be rebuilt:

- **One-finger pan**, because the stage is created `draggable: true` and Konva maps touch to drag.
- **The tools**, which already bind `touchstart.tool` / `touchmove.tool` / `touchend.tool`
  alongside their mouse equivalents in `attachToolHandlers`. Drawing and fog painting work with a
  finger today.

Only the zoom is missing.

## Work

- [ ] Two-finger pinch on the stage, scaling around the midpoint between the fingers the way
      `handleWheel` scales around the cursor, clamped to the existing `MIN_SCALE` / `MAX_SCALE`.
      Continuous rather than stepped — `ZOOM_STEP` is a wheel-notch idea and a pinch has a real
      ratio to use
- [ ] Decide what a two-finger gesture means while a tool is active (see below)
- [ ] The arithmetic in its own module under `web/src/lib/` with unit tests, alongside `measure.ts`
      and `drawing-hit.ts`
- [ ] A double-tap or a two-finger tap to reset the view would pair well with the existing
      "Reset view" button (`resetView`), but is a separate decision — don't let it hold this up

## Traps, in the order they'll be hit

**Anything sized in screen pixels has to be re-rendered after a scale change, not merely
redrawn.** `handleWheel` ends with `renderGrid()`, `renderMeasurements()`, `renderSelection()` and
`refreshCursorOverlay()` for exactly this reason — the grid, the measurement labels, the selection
ring's stroke and dashes, and the eraser's halo are all authored in screen pixels and converted
through `screenToWorld()` at render time. A pinch handler that calls `stage.batchDraw()` and stops
will leave every one of them the wrong size, and it'll look like a rendering bug rather than a
missing call. This contract is written down in
[canvas.md](../../../.claude/skills/longtable-feature/references/canvas.md).

**A tool owns the pointer, and a pinch is two of them.** `attachToolHandlers` sets
`stage.draggable(!isActive)` precisely because a tool and panning both start on a drag. A second
finger landing mid-stroke currently reads as more of the same stroke. The options are to ignore
pinch while a tool is active (simplest, and consistent with panning already being disabled), or to
let a second touch retract the in-flight gesture and take over — which is the nicer behaviour and
needs the same care `attachToolHandlers` takes when retracting a measurement mid-drag.

**The one-finger pan is a stage drag, so the second finger has to stop it.** Konva's own
multi-touch recipe calls `stage.stopDrag()` when the pinch begins, or the stage pans and scales
at once and the map slides out from under the fingers. Check `Konva.hitOnDragEnabled` and
`Konva.captureTouchEventsEnabled` while you're there — multi-touch on a draggable stage depends on
those, and their defaults have moved between Konva versions.

**Reset `lastDist` and `lastCentre` on `touchend`.** Lifting one finger of two leaves the other
mid-drag; without clearing the pinch state, the next touch jumps the scale by whatever the stale
distance was.

## Testing

Playwright's `page.touchscreen` is single-touch, so a real pinch can't be driven from the ordinary
API — it needs raw CDP `Input.dispatchTouchEvent` with two touch points, which is Chromium-only
and a first for this suite. That's the argument for putting the arithmetic in a plain module with
unit tests and keeping the handler itself thin: the part that has to be correct is then the part
that's cheap to test, and what's left is a gesture that can be checked by hand on the tablet.

## Related

- [random-id-without-secure-context](random-id-without-secure-context.md) — the other thing in the
  way of playing from a tablet, found in the same sitting. Drawing throws on any non-localhost
  origin, so touch-drawing on a tablet is broken for a reason that has nothing to do with touch.

## Blocks

[full-bleed-map-layout](full-bleed-map-layout.md) is a deliberately phone-facing redesign — the map
fills the screen and the panel becomes a bottom sheet. A map that can't be zoomed on the device the
layout is being built for would undo most of the point, so this should land before or alongside it.

## Related user stories

- [room-member-pinch-zoom-map](../../user-stories/room-member-pinch-zoom-map.md)
