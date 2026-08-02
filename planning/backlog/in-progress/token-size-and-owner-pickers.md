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
- **Owner**: the backend already has an `OwnerParticipantID` field on `Token`
  (`internal/store/token.go:24`) and `handleTokenCreate` already accepts it, but there's no UI
  control to set it, and no way to list who it could even be set to yet — this one is blocked on
  [list-room-participants](../open/list-room-participants.md) landing first.

## Related user stories

- [gm-set-token-size](../../user-stories/gm-set-token-size.md)
- [gm-assign-token-owner](../../user-stories/gm-assign-token-owner.md)
