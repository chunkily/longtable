---
title: GM sets an optional room join password
created: 2026-07-29
status: done
---

As a GM
I want to set an optional join password for my room, separate from the GM password
So that I can restrict who can join as a Player even if the room is public or the link is shared

## Acceptance criteria

- [ ] Join password is optional; if unset, anyone with the link (or who finds a public room) can join as a Player freely
- [ ] When set, a Player must enter the correct join password before joining, regardless of room visibility
- [ ] The join password is independent of the GM password (setting/changing one doesn't affect the other)
- [ ] A GM can set, change, or remove the join password at any time

The second criterion's "regardless of room visibility" is a leftover from before this item's
visibility half was dropped (see [gm-set-room-visibility](gm-set-room-visibility.md)) — there is no
room visibility any more for the password to apply "regardless" of. Read as "when set, a Player
must enter the correct password before joining, full stop," every criterion holds: the password is
optional and unset by default (`internal/store/room.go`'s `SetJoinPassword`, empty clears it), it
gates both ways of becoming a Player — a fresh seat and claiming one already sat in
(`internal/api/rooms.go`'s `joinRoom`) — but not a device resuming a session it already holds, it
shares nothing with the GM password (a separate hashed column, a separate endpoint,
`TestSetJoinPassword_DoesNotAffectGMLogin`), and a GM sets, changes or removes it at any time from
`Manage room`, with nobody signed out by the change.
