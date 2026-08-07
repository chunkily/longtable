---
title: Contextual drawing controls
created: 2026-07-29
status: done
tags: [drawing, ui]
story: room-member-contextual-drawing-controls
---

Make drawing controls like color selection, fill and range input only show when the relevant tool is selected.

**Absorbed by [full-bleed-map-layout](full-bleed-map-layout.md)** as of the 2026-08-04 design
session, which generalises this from the drawing tools to every tool. Tools group into families —
hand, draw, measure, fog, ping — and a contextual strip below the tool row carries the active
family's variants and settings and nothing else. Draw's strip holds the four shapes, the eraser,
colour and stroke width; measure's holds the four templates, size and snap; hand and ping have no
strip at all because neither has options.

Keep this item open only until that one lands. If the layout work is picked up first, close this
one out pointing at it rather than building the drawing-only version twice.

## What shipped

Exactly that: [full-bleed-map-layout](full-bleed-map-layout.md) landed first and this closes out
against it, unbuilt in its own right. The generalised version is in
`$lib/components/tool-strip.svelte` — one strip below the tool row carrying the active family's
variants and settings and nothing else. Draw's holds the four shapes, the eraser and the colour
swatches; measure's the ruler, four templates, snap mode and a line's width; fog's the two paint
tools and the two bulk actions. Hand and ping render no strip at all, since neither has options.
