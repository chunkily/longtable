# ADR-0007: The table is trusted

**Status:** Accepted
**Date:** 2026-08-05
**Deciders:** Developer

## Context

Longtable keeps making the same decision in different clothes: whether to stop a Room Member from
doing something they *could* misuse. Anyone can drag anyone's token. Deleting is undoable rather
than confirmed. Rooms aren't listed, but a link is enough to get in. Each of those was argued from
scratch, and each argument ran the same way, which is a sign the underlying decision was never
written down.

It came to a head deciding how a Player reclaims their identity on a new device
([ADR-0008](0008-seats-and-sessions.md)). The obvious hardening — a per-seat PIN — would have
worked, and would also have rebuilt the account system this project exists to avoid, one support
path at a time.

The people in a Longtable room are four to six friends who arranged to be there. They are on the
same LAN, usually in the same house, often in the same room. Someone who wants to ruin the session
can do it by talking.

## Decision

**Where a control would only defend against a Room Member behaving badly, don't build it.** Make
the action visible and reversible instead.

The line: **role boundaries are enforced, identity boundaries are not.** A GM can do things a
Player cannot — hidden tokens are withheld from Players, deleting is GM-only, a Player may change
trackers only on a token they own. Those are enforced server-side and always will be, because they
protect the *game*: a GM prepping an ambush needs it to stay secret, or the surprise isn't one.

But between two Players, or between two people who could each be at the table, Longtable does not
adjudicate. Anyone can move anyone's token. Anyone can take any unclaimed seat.

## Options Considered

### Option A: Enforce person-to-person boundaries server-side

Per-seat secrets, claim approval, per-token movement locks on by default.

**Pros:** Robust to a bad actor who has the link. Familiar from hosted VTTs.
**Cons:** Every secret needs a reset path, and a reset path needs someone with authority to run
it — which is an account system arriving sideways. Costs setup effort at every session for a
threat that, on a home LAN, mostly doesn't exist.

### Option B: Trust the table; make actions visible and reversible — CHOSEN

**Pros:** No secrets to manage, distribute or reset. Setup is nearly nil. Matches how a physical
table works, where nothing stops someone moving your miniature except that they wouldn't.
**Cons:** Someone with the link who wants to be disruptive can be. Recovery is social — the GM
sees it happen and deals with it out of band.

### Option C: Make it configurable per room

**Pros:** Serves both.
**Cons:** Doubles the paths through every feature it touches, and a setting nobody at a home table
turns on is carried by everyone forever. Deferred, not rejected — if a real use case turns up
(a convention, an open drop-in game), a room-level setting is the shape it should take.

## Trade-off Analysis

The deciding factor is that the threat Option A defends against is largely unreachable. Longtable
is self-hosted on a LAN ([ADR-0001](0001-self-hosted-multi-room.md)); getting into a room means
being given a link by someone in the group. The attacker it protects against is therefore a friend
who was invited and chose to be a nuisance — which is a social problem, and one the GM can already
solve by not inviting them again.

Against that, the cost of Option A is paid at every session by everyone: passwords chosen, shared,
forgotten and reset, before anyone rolls a die.

Reversibility does the real work here. Undo covers a misdragged token, and a GM watching the map
sees a mistake as it happens. That combination handles the accident case, which is overwhelmingly
the common one — people mostly click the wrong thing rather than plot.

## Consequences

- Explains, and should be cited by, several decisions that would otherwise look careless: dropped
  public/private rooms ([gm-set-room-visibility](../user-stories/gm-set-room-visibility.md)),
  open-claim seats ([ADR-0008](0008-seats-and-sessions.md)), anyone being able to move anyone's
  token, and preferring undo over confirmation dialogs.
- **Not a licence to skip the GM/Player boundary.** That one is about keeping the game's secrets,
  not about trust, and it's enforced on the server. A change that lets a Player see a hidden token
  is a bug, not an application of this ADR.
- **Not a licence to skip input validation or room isolation.** Nothing here is about traffic from
  outside the room. A room leaking into another room is still the most serious class of bug in
  this codebase ([ADR-0001](0001-self-hosted-multi-room.md)).
- If Longtable is ever exposed to the open internet, this ADR's premise is gone and it should be
  revisited rather than quietly relied on. `host-remote-connectivity-docs` is where that would
  first become real.

## Action Items

None directly. This documents a principle already implicit in shipped behaviour; the first thing
to cite it deliberately is [ADR-0008](0008-seats-and-sessions.md).
