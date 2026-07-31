---
title: Area-of-effect measuring tool
created: 2026-07-29
tags: [tools, map, gameplay]
story: room-member-measure-aoe
---

Measuring tool for area of effects (cones / spheres) where the affected area originates from the current mouse location.

The ephemeral broadcast this needs already exists: [measuring-tool-distance](../done/measuring-tool-distance.md)
shipped `measure.update`/`measure.end`, keyed by participant and cleaned up on disconnect, plus
a canvas layer of its own for anything that only lives for the length of a drag. A template
carries a shape and a size rather than two endpoints, so the payload grows — but the lifecycle,
the throttling and the echo handling are all the same problem, already solved.

## Scope, revised 2026-08-01

The "cones / spheres" above, and the covered-square highlighting the story used to ask for, both
changed after a read through how tables actually handle areas on a grid:

- **Four shapes, not two.** The 2024 PHB names six — Cone, Cylinder, Line, Cube, Emanation,
  Sphere — which flatten to four outlines on a top-down map. Sphere, Cylinder and Emanation are
  all a circle: a Cylinder's height is off the plane, and an Emanation differs only in being
  anchored to a creature, which matters when something persists and follows a token, not while a
  template is being dragged out. So: circle, cone, line, cube, sharing one gesture and one
  ephemeral broadcast.
- **No covered-square highlighting.** Tables disagree about which squares an area catches — a
  sphere on a cell centre or on an intersection, a 5 ft cube covering one square or four — so
  highlighting would be this app picking a side, and picking it invisibly. The template draws its
  true shape and the players read it, the way a paper cutout on a battle map always worked. The
  reasoning is in [room-member-measure-aoe](../../user-stories/room-member-measure-aoe.md) and in
  the header comment of `web/src/lib/aoe.ts`, which is where someone will look when they wonder
  why there is no coverage function.
- **A snap mode instead**, since where a template may *sit* is the thing tables actually differ
  on: free, cell centres, or intersections. Local to whoever is dragging — the points that go on
  the wire are already snapped, so none of it needs to reach the protocol.

Two shapes need more than a drag can express:

- **Line** takes a width as well as a length, so it gets a width control defaulting to 5 ft.
- **Cube** is dragged as two opposite corners rather than an origin and a size. Those two fix a
  square completely — the other pair is the same diagonal turned a quarter turn about the centre
  — so one drag sets size *and* rotation: along an axis it stands on a corner as a diamond,
  diagonally it comes out square to the grid. This puts the point of origin on a corner, which is
  still "anywhere on a face" as the PHB has it.

Obstruction ("if all straight lines from the point of origin to a location are blocked, that
location isn't included") is out of scope: it needs a walls model that doesn't exist, the same
reason vision-based fog was deferred in [fog-of-war-controls](fog-of-war-controls.md).

## What shipped

Four template tools next to the measuring ruler — circle, cone, line, cube — each dragged out
from a point of origin, filled faintly so the map reads through, labelled with its size in feet,
and live on everyone's map for as long as the drag lasts. A snap control (corners / centres /
free) and, for lines, a width control appear only while a template tool is active.

The geometry lives in `web/src/lib/aoe.ts` with 18 unit tests, and **there is no function that
answers which squares a template covers**. That's the decision to know before touching this: the
reasoning is in the module header and in the story, and it came from the observation that tables
disagree — a burst on a cell centre or an intersection, a 5 ft cube over one square or four. The
snap mode is what carries that disagreement instead, and it costs the protocol nothing because
points are snapped before they're sent.

Things that would be expensive to rediscover:

- **Templates ride `measure.update` rather than a channel of their own.** They are the same
  gesture — one per participant, replaced on each update, dropped on disconnect — so the payload
  grew a `kind` and a `widthFeet` and nothing else changed. `kind` defaults to `distance` when
  absent, which is what keeps the original ruler working untouched.
- **A prop a tool handler needs must be read inside `attachToolHandlers`, not in the closure.**
  That function runs in the `$effect` that rebinds handlers, so only synchronous reads are
  tracked. `snapMode` read inside the handler was captured once and never refreshed — the snap
  buttons appeared to do nothing until the tool was reselected. `lineWidthFeet` happened to work
  because it's read synchronously, which is exactly why only one of the two was broken and why
  the bug was easy to miss.
- **The template options row changes the page height**, so the canvas moves when a template tool
  is selected. An e2e spec that caches `canvasOrigin` across a tool change silently offsets every
  drag — and with snapping on, a short drag can round both endpoints onto the same intersection
  and draw nothing at all, which reads as "the tool is broken" rather than "the test is".
- **The rubber-band drawing branch used to be a fall-through**, so any tool added without its own
  branch quietly behaved like a drawing tool. It now names its three kinds explicitly and the
  type checker catches the omission.

Known nit, accepted: because the options row appears on selection, picking a template tool nudges
the map down. Reserving the space permanently would waste it for every other tool; it's the same
reflow the toolbar already does when it wraps.

Cylinder and Emanation are covered by the circle tool as far as a top-down map can tell them
apart. An Emanation only becomes its own thing once a template can persist and follow a token,
which nothing does yet.
