---
title: Token HP / condition tracker
created: 2026-07-29
tags: [tokens, gameplay]
---

Add controls to make it easy to track damage / hp / conditions on individual tokens.

The surface to put them on now exists: [token-detail-panel](../done/token-detail-panel.md) has
shipped `token-detail-dialog.svelte`, opened from the token details section above chat, and
`token.update` to carry the fields. What's left here is the part that item deliberately didn't
touch — the columns (three labelled numeric tracker slots plus condition tags), the fields on
`store.Token`, and adding them to the dialog and the payload.

Two things from that item that bear on this one. `token.update` sends every editable field every
time and the handler edits the loaded token in place, so adding a field means adding it to the
request struct, the assignment block and the payload — miss the assignment and it silently never
saves. And the role check is currently a single GM-only gate at the top of the handler; the
`player-set-own-token-hp-condition` story needs it to become per-field, since an owner should be
able to change HP without being able to change visibility.

## Related user stories

- [gm-set-token-hp-condition](../../user-stories/gm-set-token-hp-condition.md)
- [player-set-own-token-hp-condition](../../user-stories/player-set-own-token-hp-condition.md)
