---
title: "Map grid alignment: make it mandatory, and make it work past a handful of squares"
created: 2026-08-14
status: done
tags: [assets, ui]
story: room-member-reliably-align-large-maps
---

Two real problems with the shipped alignment step
(`web/src/lib/components/grid-aligner.svelte`, wired up from
`web/src/routes/r/[slug]/assets/+page.svelte`; see
[map-asset-grid-offset-padding](map-asset-grid-offset-padding.md), `status: done`):

1. **It's skippable, and skipped by default.** A staged map starts with `aligning: false`
   (`+page.svelte:172`) — reaching the overlay at all takes an explicit "Align to grid" click, and
   "Add to library" works without ever touching it. Every map has *some* offset, zero included;
   asking is cheap, and the current design makes it easy to upload a map nobody ever checked
   against the grid. Unaligned fog and token placement is a much harder bug to notice after the
   fact than a five-second drag would have been to do up front.
2. **The preview is unusable on anything past a small map.** The overlay renders the whole image
   inside a fixed `max-w-xs` box (`grid-aligner.svelte:91`) with no zoom. On a map with more than a
   few dozen squares across, the grid lines compress into a solid mesh — confirmed in this session
   with a ~30-square-wide tavern map at 70px squares, which rendered as an unreadable blue smear —
   and judging a few pixels of drag against that by eye is not possible.

## Direction

- Make alignment mandatory for anything filed as a map: no "Skip alignment" escape hatch, and no
  path from "Choose maps" to "Add to library" that doesn't pass through the overlay.
- Give the preview room to actually work. Dropping the `max-w-xs` cap in favour of the available
  width helps but likely isn't enough on its own; a zoom control that lets a Room Member inspect
  one corner of the grid at (near) native pixel scale while dragging is probably what's actually
  needed, since eyeballing sub-pixel offset against a whole multi-hundred-square map at once was
  never going to work.

## What shipped

**The skip is gone.** A staged map renders the aligner immediately — there is no `Align to grid`
button, because there is no longer a path past it. `aligning` is off the `Staged` interface
entirely, and `add()` now sends grid figures for every map rather than only for one somebody
opted into. An untouched map sends the defaults, which is a zero offset and the same 70px square
the scene dialog would have assumed anyway, so the honest answer and the old silent one agree —
nothing downstream had to change to absorb it.

**The preview lost its `max-w-xs` cap and gained three zoom steps** — `Fit`, `100%`, `200%` —
inside a `max-h-96 overflow-auto` frame. Fit is still the default and is the only view that
answers "do the lines keep matching across the whole map?", which is the question the square size
is set against. 100% is native pixel scale, which is the other half of the job and the half fit
physically could not do: on the ~30-square map this was reproduced against, a 70px square draws at
**17.4px** in fit, which is the solid smear the item describes.

The display maths moved to `web/src/lib/grid-zoom.ts` beside a test, following `pinch.ts` and
`pan.ts`. That was not tidiness — it is the only way to check any of this, and it caught things:

- **Scale is measured for fit and taken at face value for a number.** A numeric zoom sets the
  frame to `naturalWidth * zoom` and nothing narrows it, so at 200% the scale *is* 2. Measuring
  instead reads a width the browser hasn't laid out yet: the observer doesn't deliver until the
  next frame, so for one frame the grid is drawn at the previous zoom's scale over art that has
  already resized. An e2e test caught that as a hard failure — the overlay was still at fit's
  13.4× after a click to 200%.
- **Fit has to be measured off what the image came out as, not what it asked for.** A scrollbar
  takes width out of a `100%` without anything else changing; scaling by the asked-for width drew
  the grid 3% small, which is most of a square by the far edge of a thirty-square map.
- **The observer watches the image, not the container.** Observing the container misses exactly
  that scrollbar case, because a scrollbar changes what fits inside the box without changing the
  box — `clientWidth` moves while the observed border box doesn't, so the callback never fires.

One trap for whoever is next: **`ResizeObserver` and `requestAnimationFrame` do not run in a
hidden tab.** Verifying this by hand in a non-compositing browser shows a stale scale and looks
exactly like a bug in this code. Playwright composites, so the e2e test is the real check.

## Related user stories

- [room-member-align-map-grid-offset](../user-stories/room-member-align-map-grid-offset.md) — the
  original story; still accurately describes the offset mechanic itself, just not this gap.
- [room-member-reliably-align-large-maps](../user-stories/room-member-reliably-align-large-maps.md)
