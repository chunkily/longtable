---
title: Token owner picker in creation dialog
created: 2026-07-29
tags: [tokens, ui]
story: gm-assign-token-owner
---

Add an owner picker to `create-token-dialog.svelte`. The backend already has an
`OwnerParticipantID` field on `Token` (`internal/store/token.go:24`) and `handleTokenCreate`
already accepts it, but there's no UI control to set it — the dialog currently only has name,
image, and visibility.
