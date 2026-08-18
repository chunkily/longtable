---
title: High-contrast gridlines toggle
created: 2026-08-14
status: done
tags: [canvas, ui]
story: room-member-toggle-high-contrast-grid
---

Grid lines render at very low opacity by design —
`GRID_LINE = { light: '#00000022', dark: '#ffffff26' }` in
`web/src/lib/components/game-canvas.svelte:108` — so they read as a faint reference rather than
fighting with map art. That's the right default, but it means the grid can be close to invisible
on a busy or high-contrast map exactly when you need it most: lining up a token precisely, or
checking whether a fog rectangle lands on a cell boundary.

Add a toggle — a button near the tool row, not inside any one family since it isn't a tool, more
like the theme control — that swaps the grid to a fixed high-contrast stroke (not theme-dependent)
for as long as it's on. Persist it per browser like fog opacity (`web/src/lib/fog-opacity.ts`) and
the theme control, not per room: it's a viewing preference, not something worth syncing.

## Related user stories

- [room-member-toggle-high-contrast-grid](../user-stories/room-member-toggle-high-contrast-grid.md)

## What shipped

`Bold grid` on the toolbar's second cluster — beside undo, redo and reset view, because like those
it changes how the map is *shown* rather than what it holds. Everyone gets it, not just the GM.
The choice lives in `web/src/lib/grid-contrast.ts` and is stored per browser, the same call the
theme control and the GM's fog opacity make.

Three things worth not rediscovering:

- **The bold ruling is one fixed pair of colours, not a light/dark pair**, which makes it the
  exception to the note in `CLAUDE.md` about the grid following the scheme. What it has to stand
  out against is the map art, and the art has no idea what the page is wearing. It is a dark line
  over a pale casing: on light art the line carries it, on dark art the casing does. A single
  colour cannot do both, which is the whole reason the *faint* grid needs a pair.
- **Two screen pixels, not one, and that was measured rather than chosen.** A grid line sits
  exactly on a pixel boundary, so a 1px stroke covers half of the column either side of it — and
  each of those halves then blends with the pale casing underneath. The first version painted a
  mid-grey core at `rgba(142,142,143,234)`, which is not high contrast against anything. At 2px
  both columns fill outright: `rgba(45,45,47,252)` flanked by `rgba(255,255,255,217)`.
  **The e2e that counts opaque pixels passed on both versions** — more ink is more ink either way —
  so if you change these colours or widths, read the actual pixels back rather than the count.
  A throwaway spec dumping a scanline across the grid layer is how the grey was found.
- **The grid gets its own `$effect`**, tracking `highContrastGrid` and calling `renderGrid` alone.
  Folding it into the main `render()` would rebuild the map image, every fog cell and every token
  to change the colour of some lines — the same trade fog opacity's effect already makes.

Not done, and not asked for: the toggle has no keyboard shortcut, and it stays on the toolbar on a
phone while redo and reset view drop off into the menu. Those two have the menu to fall back on and
this has nowhere, and a small screen is where a faint grid is hardest to see.
