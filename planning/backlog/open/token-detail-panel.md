---
title: Token detail/inspector panel
created: 2026-07-29
tags: [tokens, ui]
---

Add a UI surface for viewing and editing a token's properties after creation. Today there's no
way to select a token and open its details — the only existing UI is the creation dialog
(`create-token-dialog.svelte`), and clicking a token on the canvas only supports drag-to-move.

This panel is a prerequisite for both assigning/reassigning a token's owner post-creation and for
editing HP/conditions. HP/condition stories also call for a lighter-weight hover tooltip on the
token itself (values + conditions visible on hover, without opening the full panel) — a separate,
smaller UI piece alongside the panel, not a replacement for it.

## Related user stories

- [gm-assign-token-owner](../../user-stories/gm-assign-token-owner.md)
- [gm-set-token-hp-condition](../../user-stories/gm-set-token-hp-condition.md)
- [player-set-own-token-hp-condition](../../user-stories/player-set-own-token-hp-condition.md)
