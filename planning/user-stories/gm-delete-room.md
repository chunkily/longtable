---
title: GM deletes a room
created: 2026-08-04
status: done
---

As a GM
I want to delete a room I've finished with
So that an abandoned campaign stops cluttering the server and its contents don't sit there forever

## Acceptance criteria

- [ ] A GM can delete their own room from Manage room
- [ ] Deleting asks for confirmation first — unlike everything else destructive in this app, it
      can't be undone
- [ ] Everything belonging to the room goes with it: scenes, tokens, fog, drawings, chat and the
      roster
- [ ] Images shared with other rooms survive — only this room's library entries go
- [ ] Anyone still connected is told the room is gone rather than being left on a dead socket

## Verified

All five hold, across three levels: the store test for what cascades and what survives, the hub
test for the event reaching both people and the sockets then closing, and `delete-room.spec.ts`
for the whole thing in two browsers — the GM's confirmation, the player being told, both browsers
ending up home with the room gone from their own list, and the room code answering `Room not
found!` afterwards.
