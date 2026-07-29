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
