---
title: Room Member creates several tokens at once
created: 2026-08-04
status: done
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

## Verified 2026-08-08

Every criterion holds. The second and fifth are the ones that needed the server rather than the
form: the cap is `maxTokensPerCreate` in `internal/ws/hub.go` and is checked before anything is
made, and the spreading is `spawnCells` in `internal/ws/spawn.go`, unit-tested for the cases a
screenshot can't show — footprints rather than corners, and large tokens clearing each other.

The sixth criterion reads literally and was built that way: creating eight puts eight entries on
this session's history, so Ctrl+Z takes `Monkey 8` first. `web/e2e/player-tokens.spec.ts` proves
"three squares, not a stack" by ink rather than by coordinates — three tokens on one square draw
about as much as one, which is exactly the failure the criterion is guarding against.
