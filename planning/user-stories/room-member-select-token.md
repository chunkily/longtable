---
title: Room Member selects a token to see it highlighted and its details
created: 2026-07-31
status: done
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

## Verified

This was `incomplete` from 2026-08-05 on one missing criterion: the initiative-tracker linkage,
which couldn't exist until [initiative-tracker](../backlog/initiative-tracker.md) did. That
landed, and the linkage came with it — `web/src/lib/components/initiative-panel.svelte` sets
`selectedTokenId` from an entry's own button, so picking a combatant rings the token it stands
for. The other five were already true and were re-checked rather than assumed:

- Clicking a token selects it, and anything that isn't a token — bare grid, the map image, a
  drawing — clears the selection, both off the one stage handler in `game-canvas.svelte`.
- The ring is two concentric dashed circles sharing a dash pattern, black underneath and pale
  yellow (`#fde68a`) on top, rotating slowly. The two-layer arrangement is what makes it read on
  a lit tavern floor and in a night-time dungeon both; the dash is fitted as a whole number of
  periods per circumference, which is load-bearing and explained in a comment.
- The details section is a fixed block above chat that keeps its height and goes `bg-muted` when
  nothing is selected, so the rail doesn't jump — that's the placeholder state the fourth
  criterion asks for.
- Selection is deliberately not `$state` and is never sent or stored, so it dies with a reload.

Worth knowing for whoever touches this next: selecting a token also raises it to the top of the
render stack, which this story never asked for and which arrived later. That order is local to the
browser too, and a reload puts everything back in creation order.
