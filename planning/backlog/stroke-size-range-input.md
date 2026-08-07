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
shapes, the eraser and four colour swatches. The sizing question it left open is answered: the
mobile copy of the strip lives in a horizontally scrollable bar in the sheet, so a range input
doesn't have to fit in 375px — but it is still the widest thing anyone has proposed adding, and
draw's strip is the one that would have to scroll first.
