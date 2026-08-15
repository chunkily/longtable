---
title: Stroke size range input
created: 2026-07-29
status: done
tags: [drawing, ui]
story: room-member-stroke-size-control
---

Add a range input to select stroke sizes.

Lands in the draw family's contextual strip — see
[full-bleed-map-layout](full-bleed-map-layout.md), which is where the strip comes from. Sizing it
is part of that work: the strip has to fit alongside the four shapes, the eraser and the colours,
and at 375px it docks into the mobile sheet rather than floating.

**The strip now exists** ([full-bleed-map-layout](full-bleed-map-layout.md) shipped 2026-08-07):
`web/src/lib/components/tool-strip.svelte`, `family === 'draw'` branch, currently holding the four
shapes, the eraser, a paint-bucket fill toggle and four colour swatches. The sizing question it left open is
answered: the mobile copy of the strip lives in a horizontally scrollable bar in the sheet, so a
range input doesn't have to fit in 375px — but it is still the widest thing anyone has proposed
adding, and draw's strip is the one that would have to scroll first.

## Everything below the control already exists (2026-08-14)

[shape-fill-toggle](shape-fill-toggle.md) took the whole storage and wire half of this with it,
because both columns went into one schema change:

- `drawing.stroke_width` is stored, `draw.create` accepts `strokeWidth`, `drawing.created` and
  `state.sync` carry it, and `shapeForDrawing` renders it.
- `strokeWidthOf` in `web/src/lib/drawing-hit.ts` reads it off the drawing rather than returning a
  constant, so **hit-testing already accounts for it** — a thick stroke is correspondingly easier
  to click with the eraser, with nothing further to do.
- The server clamps to `[minDrawingStrokeWidth, maxDrawingStrokeWidth]` = `[1, 32]` world pixels
  and treats 0 or absent as `defaultDrawingStrokeWidth` (3, matching `DRAWING_STROKE_WIDTH` on the
  client). Those bounds are a sanity clamp, not a considered range for a control — a width is
  drawn on a map everyone shares and a Player can't erase anyone else's work, which is the only
  reason they exist. **Picking the range the control actually offers is still open**, and is
  probably a handful of named sizes rather than a continuous slider.

So what is left is the control, a `strokeWidth` prop threaded the way `shapeFilled` is, and passing
it to `room.createDrawing`'s options — which already takes it.

Worth noting the freehand branch commits through a different call site
(`room.createDrawing(sceneId, 'freehand', freehandPoints, strokeColor)` in `game-canvas.svelte`)
than the rubber-banded shapes do, and its live preview sets `strokeWidth` on the Konva line
directly. Both need the new value or a freehand stroke will preview at one width and land at
another.

## What shipped

Three named widths — Thin, Medium and Thick, each shown as a bar of the weight it makes rather than
as a number — behind one button on the draw strip, offered to every drawing tool but the eraser.
The button carries the current width as its own bar and says it in its accessible name, so the
strip still answers "what am I drawing at?" without being opened. Picking one applies to
everything drawn afterwards, including freehand; the width goes on the wire, into the database and
back out again, so a thick line is thick for the rest of the table and still thick after a reload.

The three sat on the strip itself for about an hour first, and went behind a button on the GM's
call: a third of that row for a setting picked once a session was the wrong trade, on the strip
that scrolls first on a phone. It cost ~64px of the ~530px the draw strip took.

**Not the range input in the title.** The item's own last note said picking the range was still
open and that it was "probably a handful of named sizes rather than a continuous slider", and that
is the reading taken: every other setting on that strip is a row of discrete buttons, draw's strip
is the first one to scroll on a phone, and a width between two widths is not a decision anyone at
a table wants to make. The story's first criterion was rewritten to match rather than left
failing — see the note under it.

The three sit in `web/src/lib/stroke-width.ts` rather than in the component, so a unit test can
hold them against the server's clamp: a choice outside `[1, 32]` comes back a different number,
which draws a stroke that isn't the one the button offered and says nothing about it. `Thin` is
`DRAWING_STROKE_WIDTH`, so a browser that never touches the control draws exactly what it always
did and the strip opens with a button already pressed.

Decisions worth not rediscovering:

- **The rubber-band preview now uses the real width**, where it used to be a flat 2 — which looked
  right only because it was near the default. Its dash scales with the width (`previewDash`): a
  fixed `[6, 4]` on a 16px stroke draws a row of squares, because a dash shorter than the line is
  thick stops reading as a dash.
- **The freehand cursor ring is part of this.** It shows how wide the line will be, so
  `refreshCursorOverlay`'s effect tracks `strokeWidth` — otherwise picking a new width leaves the
  ring at the old size until the pointer moves.
- Both commit paths were touched, as the note above warned. The e2e case picks Thick _without_
  reselecting the tool, which is what would catch a width snapshotted where the handlers are bound
  instead of read where they fire — the same assertion the fill toggle carries, for the same
  reason.
- The eraser drops the control rather than showing it inert, matching Fill's rule. It takes whole
  strokes, and hit-testing already accounts for how wide the one under the pointer is.
- **It is on a popover primitive, and it is the first thing here that was.** `ui/popover` wraps
  bits-ui, which was already a dependency behind the dialog, the slider and four others — so this
  added two wrapper files and no package. It was hand-rolled first, in `room-menu.svelte`'s shape
  (a transparent full-screen backdrop button, a panel placed from the trigger's rectangle at open
  time). That version worked and shipped nothing: it moved no focus into the panel and put none
  back on the trigger afterwards, which is the half you cannot retrofit cheaply and the half a
  keyboard notices. `room-menu.svelte` had the same gap and followed it onto the primitive in the
  same session, so there is no hand-rolled popup left in the room. Its `Really leave?` arming now
  disarms off the popover's open state rather than off its own `close()`, which is what makes
  Escape and a click on the map disarm it too; `seats.spec.ts` has the case, checked by removing
  the reset and watching it fail.
- **The mobile strip sits in a box that clips.** Below `lg` the strip docks into the sheet's
  `overflow-x-auto` bar, and `overflow-x: auto` computes the other axis to `auto`. A panel
  anchored to a `relative` wrapper inside that bar is clipped by it — measured on a 375px
  viewport, the panel had a box above the strip and `elementFromPoint` at its centre returned the
  canvas. A panel with _no_ positioned ancestor inside the bar escapes, because its containing
  block resolves to the sheet wrapper outside the scroller. The popover portals to the body, so
  none of that has to hold.
- **The e2e case for the phone doesn't prove the portal**, and says so in a comment. Disabling the
  portal still passes it, for the escape-hatch reason above; what it does catch is the strip
  gaining a positioned wrapper, and it is the only case that would. It was checked by disabling
  the portal and watching it pass, which is the only way to find out that a test asserts less than
  it looks like it does.
