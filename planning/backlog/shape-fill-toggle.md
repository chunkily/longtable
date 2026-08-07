---
title: Shape fill toggle
created: 2026-07-29
status: open
tags: [drawing, tools]
story: room-member-shape-fill-toggle
---

Add a toggle to allow setting a fill on rectangle and ellipse shapes.

Lands in the draw family's contextual strip, and only when rect or ellipse is the chosen shape —
see [full-bleed-map-layout](full-bleed-map-layout.md). That strip is the mechanism
[contextual-drawing-controls](contextual-drawing-controls.md) was asking for, so this no longer
needs to solve "where does a control live that only applies to two of the four shapes" on its own.

**The strip now exists** ([full-bleed-map-layout](full-bleed-map-layout.md) shipped 2026-08-07):
`web/src/lib/components/tool-strip.svelte`, `family === 'draw'` branch. The measure strip beside
it already does the "only while the right variant is chosen" trick twice — snap mode appears once
a template is picked, and line width only for `template-line` — so copy one of those rather than
inventing the pattern. Note the strip is rendered twice, floating on a desktop and docked in the
mobile sheet, from the one component with bound props, so a control added once turns up in both.
