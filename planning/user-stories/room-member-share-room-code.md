---
title: Room Member shares the room code from inside the room
created: 2026-08-10
status: done
---

As a Room Member
I want to see this room's code without leaving the table
So that I can get a latecomer in without reading characters off my address bar

## Acceptance criteria

- [ ] The room code is visible from inside the room, without opening anything
- [ ] It's selectable, so it can be taken in one gesture rather than transcribed
- [ ] The room says how to pass it on, for someone who doesn't know a code is enough
- [ ] It works the same on a LAN address as on the GM's localhost
- [ ] A Player can do this too, not only the GM

## Notes

Anyone at the table, not just the GM: a Player is as likely to be the one messaging the friend
who's running late, and per [ADR-0007](../decisions/0007-the-table-is-trusted.md) identity
boundaries between Room Members aren't enforced. A Player can already read the code out of their
own address bar, so gating this would be theatre.

**A copy-to-clipboard button was built and then deliberately removed** — the reasoning is on
[share-room-code-from-room](../backlog/share-room-code-from-room.md) and is the most useful thing
in either file. In short: `navigator.clipboard` exists only in a secure context, and the audience
for this product is on `http://192.168.x.x:8080`. The fourth criterion above is what it failed.

That criterion is worth reading literally, and is why the story is `done` without a copy control.
It doesn't ask for a button; it asks that whatever ships behaves the same for a Player on the LAN
as for the GM on localhost. Static text does. A clipboard button, on the evidence, does not.
