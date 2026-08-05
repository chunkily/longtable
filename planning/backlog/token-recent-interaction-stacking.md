---
title: Stack recently-interacted tokens on top
created: 2026-07-31
status: open
tags: [tokens, ui]
story: room-member-token-stacking-order
---

Today a token's stacking order on the canvas is whichever order it appears in `room.tokens`
(creation order, newest last — `renderTokens` in `game-canvas.svelte` adds each token group to
`tokenLayer` in array order, and Konva draws later-added nodes on top). Older tokens stay
permanently buried under anything created after them, regardless of what a Room Member is
actually doing on the map.

Instead: any pointer interaction with a token — click or drag, not just a completed move — should
bring it to the top of the stack. That also includes selecting a token's linked entry in the
[initiative tracker](initiative-tracker.md), once that item ships — the same trigger
[token-selection-highlight](token-selection-highlight.md) uses for its selection ring, so
the two should share one bump-to-top call rather than each wiring up their own tracker hook.
Purely local and purely visual: it's each client's own recency order, not synced to the room and
not persisted, the same locality as token-selection-highlight.

That item has since shipped, and its click handler is the hook to hang this on: a single
`click.select`/`tap.select` handler on the stage that walks up from `e.target` to the group named
`token` (`findAncestor('.token', true)`). It already resolves a click to the exact Konva group
this item wants to `moveToTop()`, so there is no second hit-test to write.

Worth landing this as a `moveToTop()` on the interacted group's existing Konva node, not a
re-sort-and-rebuild of `tokenLayer`. [token-drag-causes-canvas-lag](token-drag-causes-canvas-lag.md)
just got `renderTokens` down to only rebuilding when `room.tokens` itself changes (server state) —
routing this through the same path would reintroduce a full tokens-layer rebuild for something
that never needs to touch the store.

- [ ] Local recency-ordering state, keyed by token id, separate from `room.tokens`
- [ ] Any pointer interaction on a token's group raises it via `moveToTop()`
- [ ] Selecting a token from the initiative tracker raises it the same way, once the tracker exists
- [ ] No wire message, no persistence

[player-created-tokens](player-created-tokens.md) raises the stakes here: it lets anyone spawn up
to 20 tokens in one go. They're specified to spread outward into free squares rather than stack,
partly for this reason — but a group of summons crowding one corner of the map is exactly the
case where not being able to get at what's underneath starts to hurt.

## Related user stories

- [room-member-token-stacking-order](../user-stories/room-member-token-stacking-order.md)
