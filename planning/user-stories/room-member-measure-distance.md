---
title: Room Member measures distance
created: 2026-07-29
status: done
---

As a Room Member
I want to measure the distance between two points on the map
So that I can tell how far something is without counting squares manually

## Acceptance criteria

- [ ] I can drag from a start point to an end point to see the measured distance
- [ ] Distance uses the alternating diagonal rule: the 1st diagonal step costs 1 square, the 2nd costs 2, alternating from there — not straight-line distance
- [ ] The distance is shown in feet, at a fixed 5ft per grid square, not just raw square count
- [ ] The distance updates live as I drag
- [ ] The measurement is visible to all Room Members in real time while I'm actively measuring
- [ ] The measurement disappears once I finish; it isn't persisted
