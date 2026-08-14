---
title: Stroke size range input
created: 2026-07-29
status: open
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
shapes, the eraser, a `Fill` toggle and four colour swatches. The sizing question it left open is
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
