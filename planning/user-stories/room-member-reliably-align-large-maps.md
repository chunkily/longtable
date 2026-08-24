---
title: Room Member reliably aligns a large map's grid
created: 2026-08-14
status: done
---

As a Room Member
I want map alignment to be a required step with a preview I can actually read
So that every map in the library is correctly aligned, even ones with dozens of squares across

## Acceptance criteria

- [ ] A map can't be added to the library without going through the alignment step — there is no
      skip path.
- [ ] The alignment preview stays usable regardless of map size — a Room Member can inspect
      closely enough (zoom, or equivalent) to judge pixel-level alignment on a map with dozens of
      squares across, not just a small one.
- [ ] The offset/grid-size fields and the drag gesture still agree with each other exactly as they
      do today.

## Verified

All three, against [map-grid-alignment-mandatory-and-scalable](../backlog/map-grid-alignment-mandatory-and-scalable.md).

The third is the one that needed proving rather than reading, since zoom is exactly what could
have broken it: the drag divides by the same `scale` the overlay multiplies by, so the two can
only disagree if `scale` disagrees with the picture — which it did, twice, during this work. Both
cases are covered now, by `grid-zoom.test.ts` for the arithmetic and by an e2e test for the number
that actually reaches the CSS. The existing e2e test that bakes an offset into the stored pixels
still passes unchanged, which is the end-to-end form of the same claim.
