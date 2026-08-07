---
title: Room Member takes their seat from any device
created: 2026-08-05
status: done
---

As a Room Member
I want to pick the seat I had last time when I open a room on a device that doesn't remember me
So that a cleared browser or a borrowed laptop doesn't cost me my tokens, my colour and my place at the table

## Acceptance criteria

- [ ] Opening a room on a device with no stored session shows the room's seats, and I can take one
- [ ] Taking a seat I had before restores everything attached to it — the tokens I own, my display
      name, and my colour once that exists
- [ ] I can instead say I'm new, give a name, and get a seat of my own
- [ ] A device that already has a session skips all of this, exactly as it does today
- [ ] Two devices can hold the same seat at once, and both are that one person to everyone else —
      the roster shows one entry, not two
- [ ] Taking a seat needs no password or approval; the GM seat is the exception and still needs
      the room password
- [ ] A GM can add a seat before anyone arrives, and remove one that's no longer used
- [ ] Nothing about a seat carries into another room

## Notes

The model and its reasoning are in [ADR-0008](../decisions/0008-seats-and-sessions.md); the
open-claim decision leans on [ADR-0007](../decisions/0007-the-table-is-trusted.md).

The fifth criterion is the one most likely to be quietly skipped, because it only shows up with
two devices open — which is also the case that proves a seat is a person rather than a session.
Worth testing with a real second browser, not a second tab.

"Restores my tokens" needs no migration of token rows: they point at the participant already, and
that row is the seat. Verify it by checking a token's owner still resolves after a claim from a
second browser, not by trusting that nothing needed to change.

## Verified 2026-08-07

Every criterion holds, and each is covered by `web/e2e/seats.spec.ts` using two browser
**contexts** rather than two tabs — the fifth criterion is meaningless otherwise, since tabs share
localStorage and would reuse the first device's token.

The colour half of the second criterion ("my colour once that exists") is still hypothetical:
[identity-color](../backlog/identity-color.md) is unbuilt. Nothing about it is blocked any more,
though — colour belongs on the seat, and the pre-join endpoint it was waiting for now exists.

One thing worth knowing that the criteria didn't ask for: taking a seat is offered *first* on the
join screen, above the name box, because on a device that doesn't remember you it is nearly always
what you want. Joining fresh when your old seat is sitting there is the mistake this feature was
built to stop, so the shape of the screen argues against it rather than merely allowing the right
answer.
