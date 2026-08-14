---
title: Room Member reliably aligns a large map's grid
created: 2026-08-14
status: incomplete
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
