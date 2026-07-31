---
title: Token selection highlight and details strip
created: 2026-07-31
tags: [tokens, ui]
story: room-member-select-token
---

Let a Room Member select a token — by clicking it on the canvas, or (once it exists) picking its
linked entry in the [initiative tracker](initiative-tracker.md) — and have that selection reflected
in two places: a distinct highlight on the token itself, and a small fixed section above the chat
panel showing its details. Purely local UI state, not synced over the wire or persisted; two
clients can have different tokens selected at once.

Visual for the highlight, settled by mocking up a few options first: a slow-rotating dotted ring
(~14s per rotation), pale yellow dashes over a black outline, so it stays visible on both light and
dark map backgrounds. Drop shadow and a static gold halo were the other finalists — shadow was
cheaper but less noticeable on a busy map; the rotating ring won on visibility. In Konva this is
two concentric circles in a group (thicker black stroke underneath, thinner pale yellow on top,
matching `dash`), spun by a `Konva.Animation` while the token stays selected.

Today, clicking a token on the canvas only supports drag-to-move (`game-canvas.svelte`'s
`dragend` handler) — there's no click/selection handling at all yet. This item is the one that
adds it.

Related to, but distinct from, [token-detail-panel](../in-progress/token-detail-panel.md): that
item is the full editing surface (owner, HP, conditions) and notes the same missing
click-to-select as a prerequisite. The two should share one `selectedTokenId` piece of state
rather than each growing its own — whichever lands first should leave that seam for the other.

The details section is also where the edit and delete actions live, rather than on the token
itself: an "Edit" button opens token-detail-panel's full editing surface, and a "Delete" button
is [delete-token](delete-token.md)'s. Neither is this item's to build, but the strip's layout
should leave room for them.

- [ ] Click-to-select on canvas tokens
- [ ] Rotating dotted-ring highlight on the selected token
- [ ] Fixed details section above the chat panel, with an empty state
- [ ] Wire up selection from the initiative tracker once [initiative-tracker](initiative-tracker.md) ships

## Related user stories

- [room-member-select-token](../../user-stories/room-member-select-token.md)
