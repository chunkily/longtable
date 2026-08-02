---
title: Delete a token, with undo
created: 2026-07-31
tags: [tokens, ui]
story: gm-delete-token
---

Add a `token.delete` command (GM only, persists, broadcasts `token.deleted`) — there's no way to
remove a token today at all. The button lives in the token details section above chat, which
[token-selection-highlight](../done/token-selection-highlight.md) has since **shipped** and left
room on the right for exactly this, next to the "Edit" button that opens
[token-detail-panel](../in-progress/token-detail-panel.md). Deleting the selected token also
clears the selection, same as clicking empty map space — though note that falls out for free:
the details section derives from `room.tokens`, so a token that leaves the scene already reads as
nothing selected without anything clearing `selectedTokenId`. The id does linger, which means a
token restored by undo under the same id comes back selected. That's arguably the right
behaviour here, but decide it rather than inherit it.

Undo should follow the shape [undo-redo-drawing](../in-progress/undo-redo-drawing.md) already
settled for erasing: undoing a delete is a `token.create` under the same id, the same way undoing
an erase is a `draw.create` under the same id. That reuses the existing per-session undo stack
rather than a new mechanism — this is really "give tokens the same undo the eraser already has,"
not a new kind of undo. Same caveat applies too: restoring a deleted token re-creates it under
whoever pressed undo, since the server takes authorship from the connection.

This is a second, separate remainder on top of the one undo-redo-drawing already tracks (token
*moves* still have no undo at all) — worth linking rather than merging, since move-undo needs a
position to restore and delete-undo needs a whole token recreated, different shapes of history
entry even though both are "tokens" and both ride the same stack.

- [ ] `token.delete` command + `token.deleted` event, GM only
- [ ] Delete button in the token details section, enabled only when a token is selected
- [ ] Deleting clears the current selection
- [ ] Undo restores the token (same id, position, and properties) via `token.create`

## Related user stories

- [gm-delete-token](../../user-stories/gm-delete-token.md)
