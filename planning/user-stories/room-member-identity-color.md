---
title: Room Member picks an identity color
created: 2026-08-03
status: incomplete
---

As a Room Member
I want to choose a color from a preset list for my identity in the room
So that my pings and chat messages are recognizably mine to everyone else at the table

## Acceptance criteria

- [ ] The color is chosen from a fixed preset list, not a free color picker
- [ ] The color applies to at least my pings and my name in chat
- [ ] Before choosing, I can see which colors other currently-connected Room Members have already
      picked
- [ ] Two Room Members can pick the same color — nothing blocks or warns against a duplicate, the
      visible list is only there so I can avoid one if I want to
- [ ] My color is tied to my participant record in this room, not to my device — it doesn't carry
      over to another room, or in from one, regardless of how display name ends up working
- [ ] My color persists for the rest of my time in the room, including across a dropped and
      restored connection
- [ ] It also survives me coming back on a different device and taking my seat again

The fifth criterion was unsatisfiable when this was written: `participant` had the session token
as a column, so "my participant record" and "my device" were the same row, and clearing a browser
would have lost the colour no matter how the feature was built.
[ADR-0008](../decisions/0008-seats-and-sessions.md) separates them, which is what the last
criterion above now checks. Depends on
[room-member-takes-their-seat](room-member-takes-their-seat.md).
