---
title: Room Member sees only relevant drawing controls
created: 2026-07-29
status: done
---

As a Room Member
I want drawing controls (color, fill toggle, stroke size) to only appear when a relevant tool is selected
So that the toolbar isn't cluttered with options that don't apply to my current tool

## Acceptance criteria

- [ ] The color picker only shows while a drawing tool is active
- [ ] The fill toggle only shows while the rectangle or ellipse tool is selected
- [ ] The stroke size control only shows while a drawing tool is active
- [ ] All of the above disappear when switching to a non-drawing tool (e.g. pointer/ping)

## Verified

All four hold in `web/src/lib/components/tool-strip.svelte`, and they fall out of the strip's
structure rather than being four separate checks: the whole block is inside `{#if family ===
'draw'}`, so switching family takes every drawing control with it. Inside that, the width and
colour pickers sit behind `{#if activeTool !== 'eraser'}` — the eraser is a drawing *tool* but has
no stroke to style — and the fill toggle behind `{#if activeTool === 'rect' || activeTool ===
'ellipse'}`.

Note the shipped strip carries more than this story asked for: the width and colour controls each
became a button that opens a popup, rather than sitting inline, once the colour palette grew to
two rows. That's a change in how they're presented and doesn't touch what this story asked about,
which is when they're present at all.
