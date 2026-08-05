---
title: Room Member selects a token to see it highlighted and its details
created: 2026-07-31
status: incomplete
---

As a Room Member
I want to select a token — by clicking it on the map or picking it from the initiative tracker —
and have it stand out on the map and show its details nearby
So that I can keep track of which token I'm looking at without losing it among everyone else's

## Acceptance criteria

- [ ] Clicking a token on the map canvas selects it
- [ ] Picking an entry linked to a token in the initiative tracker also selects that token
- [ ] The selected token is drawn with a slow-rotating dotted ring (pale yellow dashes on a black
      outline) so it reads against both light and dark maps
- [ ] A small fixed section above the chat panel shows the selected token's details, and shows a
      placeholder/empty state when nothing is selected
- [ ] Clicking empty map space clears the selection (ring and details section both revert)
- [ ] Selection is purely local — nothing is sent to other Room Members, and it isn't persisted
      across a reload

Marked `incomplete` as of 2026-08-05: everything is built except the initiative-tracker linkage
(second criterion), which can't exist until [initiative-tracker](../backlog/initiative-tracker.md)
does. Flip to `done` once that lands and this criterion is wired up.
