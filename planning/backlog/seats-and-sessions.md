---
title: Split session from participant, so a seat outlives a browser
created: 2026-08-05
status: done
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
- Makes the "What you can't fix from here" section of `docs/hosting.md` obsolete; update it in the
  same commit
- Pairs with [host-restores-room-access](../user-stories/host-restores-room-access.md) to make
  recovery whole: the Host gets you the link back, the seat gets you your tokens back

## Related user stories

- [room-member-takes-their-seat](../user-stories/room-member-takes-their-seat.md)

## What shipped

All of the shape above. `participant` lost `session_token` and became the seat; a new `session`
table holds tokens and points at one. `GetParticipantByToken` now joins through it and is still
the single place a credential becomes an identity, so the WS handshake, the reconnect probe and
the asset endpoints needed no changes at all — the grep the item asked for turned up one function.

New store calls: `ClaimSeat`, `CreateSeat`, `DeleteSeat`, `ListSeatsForRoom`, `DeleteSession`,
`TouchSession`. New endpoints: `GET/POST /api/rooms/{slug}/seats`,
`DELETE /api/rooms/{slug}/seats/{id}`, and `DELETE /api/rooms/{slug}/session` for leaving. Join
gained a `participantId` for claiming. The join screen leads with the seat list and keeps the old
form underneath as "I'm new here"; `Manage room` — empty until now — holds seat management.

**The migration was dropped, not written.** It existed and passed, then the whole migration layer
came out: there are no users yet, a wipe was agreed as acceptable, and `createTables` already
described every column the `addColumnIfMissing` calls were adding. `store.go` lost 214 lines
(migrations, the circle-to-ellipse rebuild, the asset-kind backfill) and `New` is now
`createTables` and nothing else. If migrations come back, they come back with a real user to
protect and a version number, not as a pile of idempotent ALTERs.

Worth keeping from the migration that was deleted, because it will be true again the next time a
table is rebuilt: **`DROP TABLE participant` with foreign keys on performs an implicit DELETE
first**, which fires `ON DELETE SET NULL` on `token.owner_participant_id` and
`drawing.created_by_participant_id`. The upgrade completed cleanly having silently un-owned every
token in every room, and the test that caught it was the one asserting a token's owner *after*
migrating. Any future rebuild of a referenced table needs `PRAGMA foreign_keys = OFF` around it —
safe here only because the pool is pinned to one connection.

Three things that fell out rather than being built:

- **Two devices on one seat is one person**, because `connectedParticipantIDs` already deduped by
  participant for the two-tabs case. The e2e for it uses two browser *contexts*, since two tabs
  share localStorage and would prove nothing.
- **A second GM login stops growing the roster.** `GMLogin` reuses the room's GM seat, so the
  password is a way into a seat rather than a way to mint a participant. That was a bug nobody had
  filed.
- **Leaving means something server-side now.** It ends that device's session and leaves the seat,
  its tokens and any other device on it alone — see [leave-room-button](leave-room-button.md),
  whose "what does leaving do beyond forgetting" question this answers properly.

Not done: `TouchSession` exists and is never called. `last_seen_at` is for a Host reading the
database, and wiring it into every request for a column nothing displays wasn't worth the write
per message.
