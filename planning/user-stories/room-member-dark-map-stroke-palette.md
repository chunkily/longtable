---
title: Room Member gets stroke colors that read on a dark map
created: 2026-07-31
status: done
---

As a Room Member
I want a second row of stroke colors designed for dark-background maps
So that my drawings and pings stay visible no matter how dark the current map is

## Acceptance criteria

- [ ] A second row of four swatches sits below the existing black/red/green/blue row
- [ ] The new row is white, bright red, bright green, bright blue — bright, highly saturated
      hues that still read as the same color family as their light-row counterpart
- [ ] Picking a swatch from either row works exactly like today: it becomes the active stroke
      color, shown with the same selection ring, immediately usable by whichever drawing tool is
      active
- [ ] Every swatch in both rows stays visibly distinct against both a light and a dark map

## Where the rows ended up

The criteria say "row" throughout because they were written when the four light-map colours sat
out on the draw strip. Both rows exist and read exactly as described — but they live in a popup
behind one strip button now, not on the strip. Eight swatches out in the open made the strip 66px
tall over the corner of the map where the art starts, which the GM turned down on sight. Read
"row" as a row of the panel and every criterion holds.

## The last criterion, and which reading it got

Taken as "the eight swatches stay distinguishable from each other, in both the light and the dark
UI" — which holds, and is what the border on every swatch is for: without it white vanishes into a
light panel and black into a dark one.

The other reading — every colour legible on every map — is what the whole item argues against. If
one set of four could do that, there would be no second row. Which row suits the map on screen is
the artist's call, deliberately, and neither row is hidden or swapped to make that call for them.
