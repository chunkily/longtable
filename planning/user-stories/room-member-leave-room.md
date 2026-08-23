---
title: Room Member leaves the room
created: 2026-07-29
status: done
---

As a Room Member
I want a button to leave the room
So that I can exit back to the homepage without closing the tab or manually editing the URL

## Acceptance criteria

- [ ] A visible "leave room" control is available while in a room
- [ ] Clicking it navigates me away from the room, back to the homepage
- [ ] Leaving ends this device's session but leaves my seat behind, so I can take the same seat again later and get my name, colour and tokens back

## A criterion was rewritten, not met

The third one originally read "Leaving is just navigation — my participant session isn't
destroyed, so I can rejoin the same room later without re-entering my details". That was written
before [ADR-0008](../decisions/0008-seats-and-sessions.md) settled what identity in a room
actually is, and the design it assumed is not the one that shipped.

`handleLeave` (`web/src/routes/r/[slug]/+page.svelte`) disconnects the socket, calls
`endSession`, clears the stored session and goes home — the session is destroyed on purpose. What
survives is the *seat*, which is the thing the original criterion was really reaching for: a
returning device picks its chair off the seat list and gets the name, the colour and the tokens
that seat owns back, without the GM having to do anything. Ending the session is what makes a
borrowed laptop cost a session rather than an identity.

So the criterion was rewritten to describe the seat rather than the session, the way
`gm-pick-map-from-library`'s were when a link-out flow replaced inline upload. The first two
criteria were always true and still are: the control is `Leave room` at the foot of the room menu,
behind a `Really leave?` confirm.
