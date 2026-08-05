---
title: Host restores a GM's access to a room
created: 2026-08-05
status: done
---

As a Host
I want to look up a room on my server and hand its link or a fresh GM password back to whoever runs it
So that a GM who has lost either isn't permanently locked out of their own campaign

## Acceptance criteria

- [ ] I can list every room on my server without needing to be in any of them
- [ ] The listing shows enough to identify a room from what a GM can tell me over the phone — its name, when it was created, and the slug that forms its link
- [ ] I can reset a room's GM password without knowing the old one
- [ ] The procedure is written down somewhere a Host will find it

## What already does this

`longtable room list` and `longtable room reset-password <slug>` in `cmd/longtable/room.go`, both
of which predate this story being written — it documents behaviour that already shipped rather
than asking for anything new. The listing prints `SLUG NAME CREATED`, which is exactly the
identify-then-hand-back path: a GM knows what they called the room and roughly when they made it,
and the slug is the answer. The procedure is in `docs/hosting.md`.

This matters more than it looks. A Host is deliberately not a GM
([roles.md](../roles.md)), so this is not someone recovering their own thing — it is the only
route back for someone else, and it is the reason
[room-member-sees-their-own-rooms](room-member-sees-their-own-rooms.md) can safely make
`localStorage` the home page's only source of truth. Losing a browser profile stops being
unrecoverable the moment a Host can read the room table.
