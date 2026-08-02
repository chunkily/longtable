---
title: Area templates in whole 5 ft steps
created: 2026-08-02
tags: [measuring, rules]
---

Dragging out an area template could produce sizes no spell has — a 7 ft cone, a 9 ft radius.
Every area in the 2024 PHB is a multiple of 5 ft, so a size between increments describes nothing
anyone can cast. Raised directly rather than coming off the backlog.

## What shipped

A template's size is rounded to the nearest 5 ft, and the *outline is rebuilt from the rounded
size* rather than the label being rounded over an unrounded shape. Origin snapping is unchanged.

The thing worth knowing before touching this again: **snapping the origin never fixed it, and
couldn't have.** Two grid corners one square apart diagonally are 5·√2 ≈ 7.07 ft, so the tidiest
possible drag in `intersections` mode still produced 7 ft. That is almost certainly where any
report of this comes from. Quantising the length is the only thing that removes it.

Three decisions, all reversible in a line if a table wants otherwise:

- **Unconditional, not a fourth snap mode.** Where an origin sits is a table convention, which is
  why that *is* a setting; what sizes exist is a rules fact. Anyone wanting an arbitrary distance
  has the ruler, which is what it is for.
- **Nearest, not floor.** Floor would mean never overstating an area, which some DMs prefer;
  nearest matches what the pointer is nearest to, which is what a drag is trying to say.
- **Clamped to one step.** Rounding to nearest alone sends anything under 2.5 ft to zero, so a
  template would wink out for the first few pixels of every drag. A drag of *nothing* still
  gives nothing, which is what keeps a shape off the map on mousedown.

A consequence worth noticing: **the far end of a template drag is no longer snapped at all.** It
can't be — quantising moves it off the grid regardless — so snapping it first would only coarsen
the direction, which is the one thing a drag genuinely expresses well. `SnapMode` therefore now
governs the origin alone, and its doc comment says so. That in turn broke the e2e snap test,
which had been asserting on total layer ink: snapping changes where the circle is centred but no
longer changes its radius, and a circle of the same size translated a few pixels covers the same
pixel count. It now probes for the origin dot, counting only *opaque* pixels — the template's
0.18-alpha fill means the probe box is inside the circle either way, so "is there any ink here"
answers yes in both cases and measures nothing.

This does not contradict `$lib/aoe`'s refusal to say which squares an area catches. That refusal
is about not adjudicating a live disagreement between tables; rounding a size to 5 ft repeats
something the rules already settled.
