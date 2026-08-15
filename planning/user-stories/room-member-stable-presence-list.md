---
title: Room Member sees a presence list that doesn't flicker
created: 2026-08-15
status: done
---

As a Room Member
I want the list of who's connected to stay still while someone's connection wobbles
So that I can tell a real departure from a bad phone signal

## Acceptance criteria

- [ ] Someone whose connection drops and comes back within the grace period never leaves the
      connected list on anyone else's screen
- [ ] Nothing is broadcast for a blip — a client watching the whole time sees no change at all
- [ ] Someone who stays away past the grace period does leave the list, for everyone
- [ ] A client that connects during someone else's grace window sees that person as connected,
      the same as everyone already in the room
- [ ] Reloading a page doesn't take the reloader off anyone's list
