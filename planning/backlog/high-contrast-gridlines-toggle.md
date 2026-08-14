---
title: High-contrast gridlines toggle
created: 2026-08-14
status: open
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
