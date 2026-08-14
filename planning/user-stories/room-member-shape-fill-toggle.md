---
title: Room Member toggles shape fill
created: 2026-07-29
status: done
---

As a Room Member
I want to toggle a fill on rectangle and ellipse shapes
So that I can draw solid shaded areas, not just outlines

## Acceptance criteria

- [ ] A fill toggle is available when the rectangle or ellipse tool is selected
- [ ] When enabled, new rectangles/ellipses are drawn filled with the selected color
- [ ] When disabled (the current default), shapes are drawn as outlines only

## Note on "solid"

The second criterion was read as **shaded in the selected colour, not opaque in it**. "Solid shaded
areas" is ambiguous between the two, and translucent is the reading that serves the goal: a drawing
is an annotation over map art people are still reading, and an opaque block would hide the terrain
it is drawn around with only an eraser to take it back. The outline stays fully solid, so a shaded
area still has a definite edge.

If someone does want to cover part of a map outright, that is what fog is for, and it is the GM's
to control.
