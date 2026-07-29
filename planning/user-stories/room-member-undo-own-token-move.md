---
title: Room Member undoes their own token move
created: 2026-07-29
---

As a Room Member
I want to undo the most recent token move I personally made
So that I can quickly correct an accidental drag without having to remember the exact prior position

## Acceptance criteria

- [ ] Undo reverts a token to its position before my most recent move of it
- [ ] Undo only reverts moves I personally made; it never undoes a move made by someone else
- [ ] Undo respects whatever move permissions already applied when I made the move (e.g. still works the same whether owner-only movement is on or off)
