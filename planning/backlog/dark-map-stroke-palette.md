---
title: Second stroke palette for dark maps
created: 2026-07-31
status: done
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

## What shipped

Eight colours — black/red/green/blue, and white/bright red/bright green/bright blue for dark map
art — **behind one button on the draw strip**, in a 4×2 grid where each dark colour sits under the
light one it answers to. The button wears the colour it will draw in, so the strip still says
which colour is loaded without being opened. Picking from either row is the same act: one
`strokeColor`, one ring, no mode.

The colours live in `web/src/lib/stroke-colors.ts`, now the only place their hex lives, and
`DEFAULT_STROKE_COLOR` from that file replaced the `'#000000'` that was written out in three
places (the strip's prop default, `game-canvas.svelte`'s, and the room page's `$state`).

**The two rows were on the strip itself first, and that is the part worth reading before putting
them back.** Four swatches in a row, then eight in two rows, exactly as this item asked for — and
the GM's answer on seeing it was that the toolbar had got too thick and was covering too much map.
It measured 66px against the 40px it replaced, floating over the top-left corner, which is where
map art usually starts. Behind a button the whole palette costs less strip than half of it did:
42px tall and 382px wide, against 444×40 before any of this. The order of that discovery matters —
the second row is what made the swatches too big to sit out in the open, so the item asking for
rows was right about the colours and wrong about where they go.

Three more things worth knowing:

- **Every swatch has a border**, and it is load-bearing rather than decoration. Without it white
  vanishes into the light panel and black into the dark one — the two ends this palette added.
  The trigger's own dot carries it for the same reason.
- **Nothing here reads the theme.** The app's scheme says what the page is wearing; the map is a
  picture, and a dark battle map under a light UI is the case this exists for. Both rows are
  always shown and the artist picks, as the item asked.
- **It is the popover primitive, not a hand-rolled panel**, for the reasons written up in
  `stroke-width-picker.svelte`: focus handling, and the portal that keeps it from being clipped by
  the bottom sheet below `lg`. `color-picker.spec.ts` covers the phone case directly rather than
  trusting that.

Duplicate values or labels would each break something real — two swatches ringed at once, or two
buttons with the same accessible name — so `stroke-colors.test.ts` guards both, along with the
row lengths matching, which is what keeps the columns paired.

The colour button now drops out for the eraser, which the swatches never did — an eraser takes
whole strokes rather than making one, so a colour there is inert. It shares the width button's
`{#if activeTool !== 'eraser'}` rather than repeating it: the two together are what a new stroke
will look like, and they should appear and disappear as one thing.
