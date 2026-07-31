---
title: Token size and owner pickers in creation dialog
created: 2026-07-29
tags: [tokens, ui]
---

Add a size picker and an owner picker to `create-token-dialog.svelte`. Broadened from
owner-only: both are additions to the same dialog (which already has name, art via the asset
library, and visibility), likely to ship in the same pass.

- **Size**: a Tiny/Small/Medium/Large/Huge/Gargantuan dropdown mapping to `width`/`height` in
  grid squares (1×1 through 4×4). The backend already accepts `width`/`height` on
  `token.create` and defaults to 1×1 when omitted (`internal/ws/hub.go`) — this is UI-only.
- **Owner**: the backend already has an `OwnerParticipantID` field on `Token`
  (`internal/store/token.go:24`) and `handleTokenCreate` already accepts it, but there's no UI
  control to set it, and no way to list who it could even be set to yet — this one is blocked on
  [list-room-participants](../open/list-room-participants.md) landing first.

## Related user stories

- [gm-set-token-size](../../user-stories/gm-set-token-size.md)
- [gm-assign-token-owner](../../user-stories/gm-assign-token-owner.md)
