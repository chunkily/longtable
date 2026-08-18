---
title: Stack recently-interacted tokens on top
created: 2026-07-31
status: done
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
- [ ] Selecting a token from the initiative tracker raises it the same way — **the tracker now
      exists** ([initiative-tracker](initiative-tracker.md), 2026-08-09), and a linked entry is a
      button that sets `selectedTokenId`. Nothing here is blocked any more; the hook to hang the
      bump on is that assignment plus the stage's `click.select` handler, which is the one call
      this item wants both paths to share
- [ ] No wire message, no persistence

[player-created-tokens](player-created-tokens.md) raises the stakes here: it lets anyone spawn up
to 20 tokens in one go. They're specified to spread outward into free squares rather than stack,
partly for this reason — but a group of summons crowding one corner of the map is exactly the
case where not being able to get at what's underneath starts to hurt.

## Related user stories

- [room-member-token-stacking-order](../user-stories/room-member-token-stacking-order.md)

## What shipped

Clicking a token, starting to drag one, or clicking its entry in the initiative tracker brings it
to the top of the stack on that screen. Nothing else moves, so a token nobody has touched keeps
the place creation order gave it. Not sent anywhere, not stored, and gone on a reload — which puts
the whole map back in creation order.

`raisedTokenIds` in `game-canvas.svelte` is the whole of the state: token ids, oldest touched
first, a plain array because nothing reactive reads it and making it `$state` would give every
effect that touched it a dependency on which token was last poked.

Two things that are easy to get wrong here:

- **`moveToTop()` alone doesn't hold.** `renderTokens` destroys and rebuilds every group in
  `room.tokens` order, and that happens whenever anyone changes any token anywhere in the room —
  so a raise made on click is gone the moment someone across the table edits a hit point. The list
  is re-applied at the end of every rebuild, which is what makes it stick. There is a spec for
  exactly this (`a raised token stays raised when the tokens are rebuilt`) because it passes
  without it as long as nothing else happens to be moving.
- **The tracker's click never reaches the canvas.** It sets the bound `selectedTokenId` from
  outside the component, so the raise hangs off an `$effect` on that id rather than off the
  stage's click handler — one path for both, as this item asked. That effect is deliberately its
  own rather than a line in the selection-ring effect beside it: that one also tracks
  `room.tokens`, so any token change would re-raise the selected token over one dragged since.

The spec proves the order by clicking where two tokens overlap and asking which one answers, since
Konva has no DOM to inspect. The overlap is a Large token with a Medium one on its top-left
square: both cover that square and the big one still has three squares of its own to be clicked
on. Two tokens the same size can't be used — the second lands on the same square as the first and
covers it completely, which leaves no way to click the buried one at all.
