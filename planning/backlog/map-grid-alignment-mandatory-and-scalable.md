---
title: "Map grid alignment: make it mandatory, and make it work past a handful of squares"
created: 2026-08-14
status: open
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

## Related user stories

- [room-member-align-map-grid-offset](../user-stories/room-member-align-map-grid-offset.md) — the
  original story; still accurately describes the offset mechanic itself, just not this gap.
- [room-member-reliably-align-large-maps](../user-stories/room-member-reliably-align-large-maps.md)
