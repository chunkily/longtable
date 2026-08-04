---
title: Shape fill toggle
created: 2026-07-29
tags: [drawing, tools]
story: room-member-shape-fill-toggle
---

Add a toggle to allow setting a fill on rectangle and ellipse shapes.

Lands in the draw family's contextual strip, and only when rect or ellipse is the chosen shape —
see [full-bleed-map-layout](full-bleed-map-layout.md). That strip is the mechanism
[contextual-drawing-controls](contextual-drawing-controls.md) was asking for, so this no longer
needs to solve "where does a control live that only applies to two of the four shapes" on its own.
