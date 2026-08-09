---
title: Token selection highlight and details strip
created: 2026-07-31
status: done
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

Related to, but distinct from, [token-detail-panel](token-detail-panel.md): that
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

- [room-member-select-token](../user-stories/room-member-select-token.md)

## What shipped

Anyone in a room can click a token to select it. The token gets the rotating dotted ring, and a
fixed section above the chat panel — desktop sidebar and mobile sheet both, it's one snippet
rendered twice — shows its name, size in squares and whether it's hidden from players, with an
empty state otherwise. Clicking anywhere that isn't a token clears it. Nothing is sent anywhere:
two people can have different tokens selected, and a reload starts with none.

The seam the other token items were waiting for: **`selectedTokenId` is a `$bindable` prop on
`game-canvas.svelte`, owned by the room page** — not `RoomClient`, which is only for things that
cross the wire. `token-detail-panel` and `delete-token` should read that same prop rather than
add their own, and their buttons go in the details section, which is laid out with room on the
right for them. ([delete-token](delete-token.md) has since shipped and done exactly that.)

Four things that would be expensive to work out again:

- **The ring is on its own Konva layer (index 8), not inside the token's group.** It's spun by a
  `Konva.Animation`, and that redraws its whole layer every frame while it runs — on the token
  layer that's a 60fps rebuild of every token, which is the same shape as the two lag bugs
  already in this folder. The cost is that nothing moves the ring for free, so `renderTokens`
  calls `moveSelectionRing` from both `dragmove` and `dragend`. The `dragend` call is not
  redundant: dropping a token back on the cell it started from is a no-op in `RoomClient`
  (see the comment on `token.moved`), so no state change arrives to re-render the ring and it
  would sit wherever the pointer let go.
- **The ring is kept alive across re-renders rather than rebuilt.** `renderSelection` runs
  whenever *anyone* moves *any* token; rebuilding would restart the animation, so the ring would
  visibly snap back to its start angle every time someone else dragged something.
- **The two circles' stroke widths are close on purpose (5 and 3).** With the black much wider
  than the yellow the whole ring just reads black — the black is meant to be an outline around a
  yellow dash, not a backing plate. Worth re-checking against a screenshot if either changes.
- **Selection only binds while `activeTool` is `'none'`.** With a tool active a click means
  erase, ping, or the first half of a drag. An existing selection survives a tool switch; it just
  can't be changed until the tool is put down. Dragging a token deliberately doesn't select it
  either, which is free — Konva suppresses `click` after a real drag.

Not done, and deliberately: the last checkbox above, wiring selection up from the initiative
tracker, since there is no tracker yet. When [initiative-tracker](initiative-tracker.md)
lands, setting `selectedTokenId` from a tracker entry is the whole of it.

**Update 2026-08-09 — that checkbox is done too.**
[initiative-tracker](initiative-tracker.md) shipped, and a linked entry in the panel is a button
that sets `selectedTokenId`; it was the one-liner predicted above, so it went in with the tracker
rather than as a return trip here. Covered by `web/e2e/initiative.spec.ts`, "clicking an entry
selects the token it stands for".

One trap that cost real time and is now written into `references/testing.md`: `page.mouse.click()`
skips every actionability check `locator.click()` makes, so the "New token" dialog — still on
screen while its exit animation ran — swallowed the click meant for the canvas, and the symptom
was indistinguishable from selection being broken.
