---
title: Second stroke palette for dark maps
created: 2026-07-31
status: open
tags: [drawing, ui]
story: room-member-dark-map-stroke-palette
---

The current stroke palette (`STROKE_COLORS`, black/red/green/blue — in `+page.svelte` when this
was written, on the draw family's strip in `tool-strip.svelte` since the full-bleed layout) was picked
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
[selected-color-focus-highlight](selected-color-focus-highlight.md)) carries over
unchanged to the new row — no new selection logic, just more swatches.

- [ ] Second swatch row: white, bright red, bright green, bright blue
- [ ] Same selection/highlight behavior as the existing row

The strip got three width buttons on 2026-08-15
([stroke-size-range-input](stroke-size-range-input.md)), so it is wider than it was when the rows
above were costed. A second row is still a row rather than more width, which is the shape that
suits it — but it is the strip that scrolls first on a phone, and this is the item that would
notice.

## Related user stories

- [room-member-dark-map-stroke-palette](../user-stories/room-member-dark-map-stroke-palette.md)
