---
title: Split session from participant, so a seat outlives a browser
created: 2026-08-05
status: open
tags: [identity, data-model]
story: room-member-takes-their-seat
---

`participant.session_token` is a column on the participant row, so a participant *is* a browser
session. Clearing browser data makes a new participant and orphans every token pointing at the old
one through `owner_participant_id`; the same person on two devices is two people.

Move the token to its own table and let a participant — the *seat* — be claimed by many sessions
over time. Reasoning and the alternatives considered are in
[ADR-0008](../decisions/0008-seats-and-sessions.md).

## Shape

- [ ] `session` table: token, `participant_id`, created/last-seen. Drop `participant.session_token`
- [ ] Migration: every existing participant becomes a seat with one session carrying its current
      token, so nobody is logged out by the upgrade
- [ ] A pre-join endpoint listing a room's seats — name, whether anyone is on it now, and later
      its colour. Careful with what it exposes: it's reachable by anyone holding the room link
- [ ] Join screen becomes a seat picker with an "I'm new here" path that behaves exactly like
      today's join
- [ ] GM can add a seat before anyone arrives and remove an unused one
- [ ] Presence separates sessions from seats — two devices on one seat is one person in the roster

## Traps

**The GM seat is a role boundary, not an identity one.** Player seats are open-claim
([ADR-0007](../decisions/0007-the-table-is-trusted.md)); the GM seat still needs the room
password. `gmLogin` already issues a session against a participant, so it becomes the same
mechanism rather than a special case — that's a simplification available here, not extra work.

**Reconnect goes through the session token, and so does the WS handshake.** Grep every read of
`session_token` before assuming the join endpoint is the only caller;
`GetParticipantByToken` is the one to follow.

**Don't let the seat list become a room-enumeration hole.** It's scoped to a room someone already
has the link for, which is fine, but it should stay that way — see
[home-page-lists-your-rooms](home-page-lists-your-rooms.md), which is busy removing exactly this
kind of listing elsewhere.

**Two devices on one seat is the case that will be missed.** It only appears with a second real
browser, and it's the one that proves a seat isn't just a renamed session.

## Unblocks

- [identity-color](identity-color.md) — colour belongs on the seat, and its unresolved "where does
  picking happen" question was waiting on the pre-join endpoint this adds
- Makes the "What can't be recovered" section of `docs/hosting.md` obsolete; update it in the same
  commit
- Pairs with [host-restores-room-access](../user-stories/host-restores-room-access.md) to make
  recovery whole: the Host gets you the link back, the seat gets you your tokens back

## Related user stories

- [room-member-takes-their-seat](../user-stories/room-member-takes-their-seat.md)
