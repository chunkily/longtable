---
title: Token detail/inspector panel
created: 2026-07-29
tags: [tokens, ui]
---

Add a UI surface for viewing and editing a token's properties after creation. Today there's no
way to select a token and open its details — the only existing UI is the creation dialog
(`create-token-dialog.svelte`), and clicking a token on the canvas only supports drag-to-move.

This panel is a prerequisite for both assigning/reassigning a token's owner post-creation and for
editing HP/conditions.

Shelved: showing values on hover, and opening this panel directly from hovering or otherwise
interacting with the token. Instead the panel opens from an explicit button in the token details
section above chat (see [token-selection-highlight](../open/token-selection-highlight.md)) — it
only ever opens on a deliberate click, once a token is already selected there, never from a
hover.

See also token-selection-highlight, which adds the same missing click-to-select and should share
its `selectedTokenId` state rather than this panel growing its own. Also see
[delete-token](../open/delete-token.md), which adds a second button in that same details
section.

## Related user stories

- [gm-assign-token-owner](../../user-stories/gm-assign-token-owner.md)
- [gm-set-token-hp-condition](../../user-stories/gm-set-token-hp-condition.md)
- [player-set-own-token-hp-condition](../../user-stories/player-set-own-token-hp-condition.md)
