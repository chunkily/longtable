---
title: Distance measuring tool
created: 2026-07-29
tags: [tools, map]
story: room-member-measure-distance
---

Add measuring tools. Distance is measured using the rule of first diagonal movement is 1, second diagonal movement is 2.

## What shipped

A "Measure" tool in the map toolbar, open to everyone: drag from one point to another and a
dashed line with a `NN ft` label follows the pointer. The rule lives in `web/src/lib/measure.ts`
— diagonals cost 1, 2, 1, 2… so a 4-cell diagonal is 6 squares, and squares are converted at a
fixed 5ft. Everyone else in the room sees the line as it's dragged, labelled with the measurer's
name, and it disappears when the drag ends.

Three decisions worth knowing before extending this — the area-of-effect tool in
[measuring-tool-aoe](../in-progress/measuring-tool-aoe.md) is the obvious next user of all
three:

- The wire carries two endpoints and nothing else. Distance is derived on each client from the
  scene's grid size, so the number and the line can't disagree, and there's no second source of
  truth to keep in step.
- `measure.update` / `measure.end` are fire-and-forget the way `ping` is — never persisted,
  never in `state.sync` — but keyed by participant, since a measurement is a continuous gesture
  and every update replaces that participant's last. The line is drawn cell-centre to
  cell-centre rather than pointer to pointer, so it agrees with the square count in its own
  label.
- The local line follows the pointer exactly while the wire is paced separately (trailing-edge
  throttle, `MEASURE_SEND_INTERVAL_MS`), and a client ignores the echo of its own measurement —
  otherwise a throttled update arriving late would drag your own line back to where the pointer
  used to be.

The one piece of server-side bookkeeping this needed: the hub broadcasts `measure.ended` when a
connection drops. A client that disconnects mid-drag never sends one itself, and its line would
otherwise hang on every other map until the scene changed.

## Related user stories

- [room-member-measure-distance](../../user-stories/room-member-measure-distance.md)
