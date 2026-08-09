---
title: Room Member views the initiative tracker
created: 2026-07-29
status: done
---

As a Room Member
I want to see the initiative tracker and whose turn it currently is
So that I know the turn order and when it's coming up on me, without asking the GM

## Acceptance criteria

- [ ] I can see the full initiative order, whose turn it currently is, and the round number
- [ ] An entry linked to a hidden token is not shown to me, consistent with that token's existing visibility setting
- [ ] A freestanding entry marked hidden by the GM is not shown to me either
- [ ] The tracker updates in real time as the GM makes changes or advances turns

## Verified 2026-08-09

All four hold, covered by `web/e2e/initiative.spec.ts` with two browser contexts and by the
per-recipient tests in `internal/ws/initiative_test.go`.

The second and third criteria are the ones worth reading literally: a withheld entry is **never
sent**, not filtered out on arrival, so counting the rows can't tell a Player how many things are
waiting in the dark. The two kinds of hidden have one answer each — a freestanding entry carries
its own flag, and a linked one's visibility is read from its token rather than copied, so the
tracker and the map cannot disagree.

One thing the criteria didn't ask about and someone will: `currentEntryId` is still sent when it
names an entry the Player can't see. Their tracker then highlights nothing, which says "it's
somebody's turn and not yours" without saying whose — the alternative, suppressing the id, would
have been indistinguishable from the encounter not having started.
