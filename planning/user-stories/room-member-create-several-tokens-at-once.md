---
title: Room Member creates several tokens at once
created: 2026-08-04
status: incomplete
---

As a Room Member
I want to say how many copies of a token to make
So that eight conjured monkeys or six goblins take one trip through the dialog instead of eight

## Acceptance criteria

- [ ] The new-token dialog has a count, available to GMs and Players alike
- [ ] The count can't exceed 20, and the server refuses more than 20 however the request was made
- [ ] A count of one creates a token named exactly what I typed, with no number appended
- [ ] A count above one numbers them from 1 — `Monkey 1` through `Monkey 8`
- [ ] They land on separate squares spreading out from the spawn point, not stacked on one square
- [ ] Undo removes them one at a time, newest first, rather than all at once
