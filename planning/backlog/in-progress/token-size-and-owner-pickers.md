---
title: Token size and owner pickers in creation dialog
created: 2026-07-29
tags: [tokens, ui]
---

Add a size picker and an owner picker to `create-token-dialog.svelte`. Broadened from
owner-only: both are additions to the same dialog (which already has name, art via the asset
library, and visibility), likely to ship in the same pass.

- **Size**: **the control already exists** —
  [token-detail-panel](../done/token-detail-panel.md) needed the same one and built it as
  `web/src/lib/components/token-size-picker.svelte`, with a `squares` bindable and a
  `sizeForSquares` helper for reading a stored token back. Wiring it into
  `create-token-dialog.svelte` and passing `width`/`height` on `token.create` is all that's left
  here; the backend already accepts them and defaults to 1×1 when omitted. Note it offers four
  options rather than six: Tiny, Small and Medium are all 1×1, and three options meaning the same
  thing can't round-trip when a token is read back, so they share one option carrying all three
  names.
- **Owner**: **no longer blocked.** `handleTokenCreate` has always accepted
  `ownerParticipantId`, and [list-room-participants](../done/list-room-participants.md) has since
  shipped the candidates: `client.participants` is the whole roster — everyone who has ever
  joined, which is the right list here rather than `connectedParticipants`, since a GM prepping
  tokens before a session is assigning them to people who aren't online yet. All that's missing
  is the control. `token.update` will take an owner the same way once it has a field for it (a
  line in the request struct and a line in the assignment block — see
  [token-detail-panel](../done/token-detail-panel.md)).

## Related user stories

- [gm-set-token-size](../../user-stories/gm-set-token-size.md)
- [gm-assign-token-owner](../../user-stories/gm-assign-token-owner.md)
