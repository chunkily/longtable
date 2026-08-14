---
title: Token drag distance preview
created: 2026-07-29
status: done
tags: [tokens, map, ui]
story: room-member-drag-token-distance-preview
---

While dragging a token, show a translucent ghost of it at the original position, a line to the
current dragged position, and the distance in feet to where it'll snap. Shares the same distance
calculation (5ft/square, alternating diagonal rule) as the standalone distance measuring tool —
see [measuring-tool-distance](measuring-tool-distance.md).

## What shipped

Exactly that. Picking a token up leaves a faded copy of it on the square it came from, a dashed
line runs from there to the square it would land on, and the ruler's own badge floats above that
square with the distance in it. All three go the moment the button is released.

Decisions worth not rediscovering:

- **Nothing about it goes on the wire.** No command, no event, no store table — the whole feature
  is `web/src/lib/token-drag.ts` plus three functions and one `applyViewChange` line in
  `game-canvas.svelte`. The story only ever asked for the dragger to see it ("So that I can tell
  how far I'm moving a token before I let go"), and a line and a label per person mid-drag would
  make a busy fight unreadable. There's an e2e case asserting a second client sees nothing, because
  a later change that started broadcasting it would otherwise look like an improvement.
- **`snapTokenCell` is shared with `dragend`.** The rounding was one expression written once, in
  the drop handler; a preview that computed the destination a second time is precisely how a
  preview starts lying about where a token will land, which is the one thing this feature exists to
  be trusted about. Extracting it is most of why `token-drag.ts` exists at all.
- **It rounds where `cellAt` floors**, and that isn't a style difference. A token's stored position
  is the cell its *top-left corner* occupies, so the question is which grid line the corner is
  nearest — flooring it, as the ruler correctly does for a pointer, drops every token a square up
  and left of where it looks.
- **The ghost is `group.clone()`.** A token's art is an image, a placeholder circle with initials,
  or a placeholder with an image still loading behind it; rebuilding that branch for the ghost is
  how the two start disagreeing. The clone carries the drag and hover handlers with it, so it gets
  `listening: false` and `draggable: false` on top of the preview layer already not listening.
- **The label hangs off the top edge of the destination square, not its centre.** Anchored to the
  centre — which is what the ruler does, since a ruler's endpoint is bare map — it clips the corner
  of a 1×1 and sits dead in the middle of a 3×3. Found by looking at a screenshot; the pixel test
  was perfectly happy with it.
- **`dragend` clears the overlay itself rather than leaving it to a re-render.** Dropping a token
  back on the square it started from is a no-op in `RoomClient`, so no state change arrives and
  nothing forces a render — the ghost would strand on the map for the rest of the session. That case
  has its own e2e test. `retractInFlightGesture` clears it too, for the pinch that stops Konva's
  drag without a `dragend` ever firing.
- Distance and geometry are unit-tested (`src/lib/token-drag.test.ts`); the e2e spec covers where
  the overlay is and when it goes away, since text painted into a canvas can't be read back.

Two things fixed in passing, both pre-existing and neither caused by this:

- `watchInkAt` in `e2e/fixtures/map.ts` kept its result in one pair of page globals, so two watches
  on the same page clobbered each other — the second reset the first's result and the first
  `stop()` switched both sample loops off. It failed in the dangerous direction: a spec asserting
  "this never appeared" passes when its watcher was turned off early. Each watch takes its own slot
  now.
- `e2e/drawing-right-click.spec.ts`'s "right-dragging the fog tool reveals nothing" was failing on
  `main`, deterministically, since [right-click-pan](right-click-pan.md) shipped. A right-drag pans
  the map now, which slides the fog rectangle across the viewport and changes the total alpha on
  screen without a cell having been revealed — so the assertion was measuring the pan. It resets the
  view before comparing.

## Related user stories

- [room-member-drag-token-distance-preview](../user-stories/room-member-drag-token-distance-preview.md)
