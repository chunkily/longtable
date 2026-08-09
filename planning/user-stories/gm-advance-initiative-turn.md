---
title: GM advances the initiative turn
created: 2026-07-29
status: done
---

As a GM
I want to advance to the next or previous turn, and track the current round
So that everyone at the table can see whose turn it is without me tracking it separately

## Acceptance criteria

- [ ] The tracker highlights whose turn it currently is
- [ ] Advancing to the next turn wraps back to the top of the order and increments the round counter after the last entry's turn
- [ ] Going back to the previous turn decrements the round counter if it moves back before the first entry
- [ ] The current round number is displayed

## Verified 2026-08-09

All four hold. The second and third are one rule rather than two — the round changes *only* at the
wrap, in either direction — which is what makes next-then-previous land exactly where it started
across a round boundary. `advanceTurn` in `internal/ws/initiative.go` is that rule on its own,
away from the handler, with the round floored at 1 and the first press of Next starting at the top
of the order rather than counting a round nobody has played.
