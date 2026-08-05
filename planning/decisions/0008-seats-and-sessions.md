# ADR-0008: Identity in a room is a seat, not a browser

**Status:** Accepted
**Date:** 2026-08-05
**Deciders:** Developer

## Context

Longtable has no user accounts, deliberately — that constraint predates the planning tree and is
one of the reasons the project exists. Identity in a room is a session token in `localStorage`.

The trouble is where that token lives. `session_token` is a **column on the `participant` row**,
so a participant *is* a browser session, and the two can't be told apart:

- A Player who clears their browser data rejoins as a **new** participant. Their old row stays in
  the roster, and every token pointing at it via `owner_participant_id` stays pointing at a person
  who can no longer log in. A GM has to reassign them by hand.
- The same human on a phone and a laptop is two participants, two roster entries, two identities.
- Rosters accumulate duplicate names over a campaign, one per device event.

This surfaced while documenting Host-assisted recovery: a GM who loses a link or password can be
restored by their Host, but a Player has no equivalent, and there was nowhere to put one that
didn't start to look like an account.

Note that a GM already has what Players lack. `gmLogin` takes the room password and issues a
session — which is exactly "prove who you are on a device that doesn't remember you". The model
existed; it just wasn't available below the GM role.

## Decision

**Split the durable identity from the device credential.** `participant` keeps the display name,
role, and everything tokens reference, and becomes the *seat*. A new `session` table holds tokens
and points at a seat, many-to-one over time.

Joining a room means **taking a seat**: pick an existing one, or make a new one. A returning
device with no stored session picks its old seat and gets its tokens, colour and history back.

**A seat is a person, not a character.** A Player can already own several tokens, and characters
die more often than players do.

**Seats are open-claim.** No secret, no approval, per [ADR-0007](0007-the-table-is-trusted.md).
Anyone with the room link can take any seat. The GM seat is the exception and stays behind the
room password, which is a role boundary rather than an identity one.

## Options Considered

### Option A: Leave it; GM reassigns tokens after a device change

**Pros:** Free. Already works.
**Cons:** Puts recurring manual work on the GM for something that isn't their mistake, and quietly
grows the roster forever. It's the status quo, and the status quo is what prompted this.

### Option B: A recovery code issued to each Player on join

**Pros:** No GM involvement; works across devices.
**Cons:** A password with no username. Another secret to lose — and the person most likely to lose
it is precisely the person who just lost their browser data. Needs a reset path, which needs an
authority, which is an account system.

### Option C: Split session from participant; joining claims a seat — CHOSEN

**Pros:** The durable half already exists and is already what tokens reference, so this is a
smaller change than it sounds. Multi-device falls out for free — two sessions, one seat. Fully
backwards compatible: "I'm new here" is today's join. Retroactive — orphaned participants in
existing rooms become claimable seats, and their tokens reattach when someone takes the name.
**Cons:** Anyone with the link can take any seat. Presence gets slightly more complicated, since
"connected" becomes a property of a session while the roster stays a property of the seat.

## Trade-off Analysis

Option C won mostly on the shape of the change rather than the feature: `participant` is *already*
the seat in every respect except that a credential is stapled to it. Removing the staple is a
smaller and more honest change than adding a parallel concept beside it, and it makes the existing
data model mean what it looks like it means.

The open-claim cost is real but bounded by [ADR-0007](0007-the-table-is-trusted.md): taking
someone's seat requires the room link, which requires being invited, and the GM watches the roster
in real time. Option B's secret would have defended against a stranger who cannot reach the
server anyway, at the cost of a credential lifecycle.

Worth being explicit, because it's the obvious objection: **a seat is not an account.** It is
scoped to one room, carries no credential, grants nothing anywhere else, and cannot be used to
identify the same human in a different room. The thing a seat remembers is which chair you sat in,
not who you are.

## Consequences

- `participant.session_token` moves to a `session` table. Every read of a session token — join,
  reconnect, and the WS handshake — goes through it, and `owner_participant_id` becomes
  meaningfully durable for the first time.
- The join screen changes from a name box to a seat picker with an "I'm new here" option, and
  needs a pre-join endpoint listing a room's seats.
- That endpoint is also what
  [room-member-identity-color](../user-stories/room-member-identity-color.md) needed and didn't
  have — its "where does picking happen" question was blocked on exactly this. **Colour belongs
  on the seat**, which is what makes that story's "tied to my participant record, not to my
  device" criterion achievable rather than nominally satisfiable.
- Presence must distinguish sessions from seats: two devices on one seat is one person in the
  roster and arguably one entry in the connected list.
- Makes the "What can't be recovered" section of `docs/hosting.md` obsolete when it ships; that
  section documents exactly this gap.
- Weakens, without killing,
  [room-member-reusable-display-name](../user-stories/room-member-reusable-display-name.md) — you
  don't type a name to take an existing seat, so a device-level name becomes a prefill for the
  "I'm new here" path rather than the main event.
- Seats accumulate across a campaign and need removing, which is the same gesture as
  [leave-room-button](../backlog/leave-room-button.md).

## Action Items

- [seats-and-sessions](../backlog/seats-and-sessions.md) carries the work.
