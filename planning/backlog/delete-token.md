---
title: Delete a token, with undo
created: 2026-07-31
status: done
tags: [tokens, ui]
story: gm-delete-token
---

Add a `token.delete` command (GM only, persists, broadcasts `token.deleted`) — there's no way to
remove a token today at all. The button lives in the token details section above chat, which
[token-selection-highlight](token-selection-highlight.md) has since **shipped** and left
room on the right for exactly this, next to the "Edit" button that opens
[token-detail-panel](token-detail-panel.md). Deleting the selected token also
clears the selection, same as clicking empty map space — though note that falls out for free:
the details section derives from `room.tokens`, so a token that leaves the scene already reads as
nothing selected without anything clearing `selectedTokenId`. The id does linger, which means a
token restored by undo under the same id comes back selected. That's arguably the right
behaviour here, but decide it rather than inherit it.

Undo should follow the shape [undo-redo-drawing](undo-redo-drawing.md) already
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

- [gm-delete-token](../user-stories/gm-delete-token.md)

## What shipped

A GM selects a token and presses Delete in the details section above chat; it goes for everyone,
persists, and Ctrl+Z (or the toolbar Undo) brings it back in the same square with the same
properties. Players get no button at all — they can still select a token and read its details.

Decisions worth not rediscovering:

- **`token.delete` is GM-only, deliberately not `draw.delete`'s "your own work" rule.** A token
  has no author to fall back on: it's a piece of the GM's scene that a Player may merely be
  allowed to *move*. Matching who may create one was the simplest defensible line.
- **The deletion of a hidden token is withheld from Players**, through the same
  `broadcastPerClient` filter its creation uses. They were never told it existed, and an id they
  have never seen turning up in a deletion is itself the leak.
- **`token.create` now takes an optional `tokenId`**, through the same `isCanonicalUUID` check
  `draw.create` uses. Nothing renders ahead of the server here — that's not what the id is for —
  but a restored token has to come back under the id the rest of the room still knows it by.
- **The delete is *not* optimistic**, unlike the eraser. A token goes via a button rather than a
  gesture, so there's no preview shape that would blink off and back on while the round trip
  happens, which is the only reason drawings render ahead of the server. `token.move` already
  waits the same way.
- **"Deleting clears the selection" needed no code.** The details strip derives from
  `room.tokens`, so a token leaving the scene already reads as nothing selected — on every path,
  including a redo, another client's delete, and a scene change. The question left open above is
  now decided: **the id is deliberately not cleared**, so undoing a deletion brings the token
  back selected. Clearing it by hand would have meant doing so at three call sites and still
  getting the redo case wrong.
- **No confirmation dialog**, unlike deleting a scene. The deletion is undoable, which is a
  cheaper answer to a misclick than a dialog on every deliberate one.

One thing found while testing that is a property of the canvas rather than of this feature, now
written into `references/canvas.md`: **a click on a token can be silently lost.** Konva only
fires `click` when `mousedown` and `mouseup` land on the same node, and `renderTokens` rebuilds
every token group whenever `room.tokens` changes — so a `token.moved` echo arriving mid-click
swallows it. About a frame wide in real use, but a click sent straight after a drag hits it three
runs in four, which is why `token-delete.spec.ts` selects *before* dragging. Fixing it properly
means diffing the token layer rather than rebuilding it, which the canvas notes argue against
elsewhere; left as a known caveat rather than quietly worked around.
