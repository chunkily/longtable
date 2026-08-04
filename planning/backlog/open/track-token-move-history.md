---
title: Track token move history
created: 2026-07-29
tags: [tokens, data-model]
story: room-member-undo-own-token-move
---

Persist enough move history to support undo. Today `MoveToken` does a plain
`UPDATE token SET x=?, y=? WHERE id=?` (`internal/store/token.go:44-47`) — it overwrites the
position in place with no record of where the token was before, and the `token.moved` WS event
only broadcasts the new absolute position, not a delta or the prior position.

Needs a move log (at minimum: token ID, mover's participant ID, previous position, timestamp) so
an undo can revert to the position before a specific Room Member's most recent move.

**The undo this was written for has shipped without it.**
[undo-redo-drawing](../done/undo-redo-drawing.md) covers token moves as of 2026-08-04, keeping the
prior position on the client's own per-session stack the way it already did for drawings — so
`MoveToken` is still a plain overwrite and `token.moved` still carries only the new position. That
was the cheaper half and it satisfies
[room-member-undo-own-token-move](../../user-stories/room-member-undo-own-token-move.md) in full.

What a server-side log would still buy, and what this item is now really about: undo surviving a
reload or a reconnect (the stack is cleared on every `state.sync`), and any GM-facing "who moved
what" view. Neither is asked for by a story yet. Worth deciding whether this is still wanted
before building it.
