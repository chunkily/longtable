---
title: Room Member sees their own rooms on the home page
created: 2026-08-05
status: done
---

As a Room Member
I want the home page to show the rooms I'm actually in, and a box to type a room code into
So that I can get back to my game without hunting through everyone else's, and without anyone seeing mine

## Acceptance criteria

- [ ] The home page lists only rooms this browser has joined or created, newest use first
- [ ] Each row says which room it is and whether I'm its GM or a Player there
- [ ] No room I haven't joined appears anywhere in the web UI, for anyone
- [ ] I can join a room by typing its room code
- [ ] I can remove a room from my own list without affecting the room or anyone else's list
- [ ] With no rooms yet, the page offers the two ways into one rather than showing an empty list
- [ ] Creating a room puts it straight into my list

## Notes

Replaces [gm-set-room-visibility](gm-set-room-visibility.md) and
[visitor-browse-public-rooms](visitor-browse-public-rooms.md); the reasoning for preferring this
shape is on the first of those.

"Rooms I'm in" is already stored — `web/src/lib/session.ts` keeps a session per room slug in
`localStorage`, so this is a read of state the product maintains rather than a new concept, and
needs nothing added server-side. That is most of why it won.

Two consequences to take on the chin rather than design around. It is per-browser, not per-person:
the same GM on a phone and a laptop sees two different lists, which is already true of identity
here and follows from there being no accounts. And clearing browser data empties the list — for a
Player that costs a message to their GM, and for a GM it is
[host-restores-room-access](host-restores-room-access.md), which is why that story was written
down before this one gets built.

The removal criterion is the same gesture as
[leave-room-button](../backlog/leave-room-button.md); whichever lands first should do both, since
"leave this room" and "take it off my list" are one action to the person doing it.

**Amended 2026-08-10**, by [home-page-welcome-screen](../backlog/home-page-welcome-screen.md). The
empty-state criterion was first read as "write better words in the empty card", and the card was
still the biggest thing on a first-time visitor's screen. It is now a welcome screen with the two
actions on it and no list at all, which is what the criterion was reaching for. The other change is
vocabulary: **room code**, not "invite link" — the wording here and everywhere public moved
together.

**Amended again the same day**: the join criterion said "pasting a link or typing its room code",
and the link half is gone. The box is one large six-character field now, and `parseRoomCode`
refuses anything else — deliberately, since a link is already a link and following it lands you in
the room without this field. The criterion above is rewritten rather than left standing with a
note, because it describes an affordance that was removed on purpose rather than one nobody has
built yet. `room-code.ts` carries the reasoning next to the code.
