---
title: Eraser tool
created: 2026-07-29
tags: [drawing, tools]
---

Add in an eraser tool to remove drawings.

## What shipped

An "Erase" tool in the map toolbar, open to everyone: click a stroke to remove it for the whole
room. Backed by a `draw.delete` command and a `drawing.deleted` broadcast, with the permission
check server-side — a GM can erase anything, a Player only drawings they authored (see
[track-drawing-creator](track-drawing-creator.md)). Drawings with no recorded author are
GM-only to erase. The client also checks before sending, so clicking someone else's work is a
no-op rather than an error toast.

Two things worth knowing if this area gets touched again:

- The eraser finds its target by hit-testing the Konva drawings layer, so that layer now
  listens for pointer events and its shapes carry a fat `hitStrokeWidth` — a 3px stroke is
  otherwise nearly unclickable.
- `renderDrawings` uses `draw()` rather than `batchDraw()` on purpose: batching defers the hit
  graph by a frame, and since every re-render rebuilds these shapes, a click in that gap
  hit-tests against destroyed shapes and finds nothing.

## Related user stories

- [gm-erase-any-drawing](../../user-stories/gm-erase-any-drawing.md)
- [player-erase-own-drawing](../../user-stories/player-erase-own-drawing.md)
