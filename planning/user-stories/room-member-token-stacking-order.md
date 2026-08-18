---
title: Room Member sees recently-interacted tokens on top
created: 2026-07-31
status: done
---

As a Room Member
I want the token I just clicked or dragged to sit above the others
So that overlapping tokens don't keep getting buried under ones I'm not currently working with

## Acceptance criteria

- [ ] Any pointer interaction with a token (click, drag, etc.) brings it to the top of the
      stacking order on the canvas
- [ ] This also extends to tokens selected via the initiative tracker.
- [ ] A token that hasn't been interacted with keeps its current position in the stack
- [ ] The stacking order is local to my own session — it isn't sent to other Room Members and
      isn't persisted across a reload

## Verified

All four hold. A click and the start of a drag both raise, which between them are every pointer
gesture a token has — Konva suppresses the click after a real drag, so neither covers the other.
The tracker's entries set the same selection the canvas does and go through the same raise.
`token-stacking.spec.ts` covers the four criteria in order, including the two-browser check that
one screen's order never reaches anyone else's and the reload that throws it away.
