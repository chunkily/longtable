---
title: Second stroke palette for dark maps
created: 2026-07-31
tags: [drawing, ui]
story: room-member-dark-map-stroke-palette
---

The current stroke palette (`STROKE_COLORS` in `+page.svelte`, black/red/green/blue) was picked
against light maps and mostly disappears on a dark one — black most of all, but the saturated
red/green/blue aren't much better. Add a second row of swatches underneath, for dark-background
maps: white, bright red, bright green, bright blue.

Colors settled by mocking up a few options first. Pastel tints of each hue were tried and
rejected — legible, but washed-out and less clearly "the same color" as their light-row
counterpart at a glance. Landed on bright/highly-saturated instead:

- White `#ffffff`
- Bright red `#ff3b30`
- Bright green `#00e676`
- Bright blue `#2979ff`

The rows are additive, not a toggle — both are always visible, so it's the artist's call which
row suits the current map rather than something Longtable has to detect. That sidesteps guessing
wrong on a busy or mixed-tone map, at the cost of a slightly taller swatch area (see
[contextual-drawing-controls](contextual-drawing-controls.md), already tracking that the drawing
controls take up space and only need to show while a drawing tool is active).

The existing selection treatment (`aria-pressed` + outline ring, from
[selected-color-focus-highlight](../done/selected-color-focus-highlight.md)) carries over
unchanged to the new row — no new selection logic, just more swatches.

- [ ] Second swatch row: white, bright red, bright green, bright blue
- [ ] Same selection/highlight behavior as the existing row

## Related user stories

- [room-member-dark-map-stroke-palette](../../user-stories/room-member-dark-map-stroke-palette.md)
