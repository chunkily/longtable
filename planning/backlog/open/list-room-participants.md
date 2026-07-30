---
title: List a room's participants
created: 2026-07-30
tags: [rooms, data-model]
---

Add a way to list everyone who has ever joined a room — `SELECT * FROM participant WHERE
room_id = ?`, exposed over the WS protocol (likely folded into `state.sync`, the way tokens and
drawings already are). Nothing in the store or protocol currently does this at all: connectivity
is tracked only in-memory, in `Hub.rooms[roomID]`, which knows who's connected right now but
nothing about the room's roster as a whole.

Found while scoping [token-size-and-owner-pickers](../in-progress/token-size-and-owner-pickers.md):
an owner picker needs to offer every Room Member who could plausibly own a token, including a
Player who joined last week and isn't online at the moment a GM is prepping tokens before a
session — that's a different list from "who's connected right now."

Keep this distinct from [connected-players-list](connected-players-list.md), which wants the
live subset. That feature will likely still need this roster as its base, with the
currently-connected ones (from `Hub.rooms`) highlighted or filtered on top of it — but the two
have different sources of truth and shouldn't be conflated into one query.

## Related user stories

- [gm-assign-token-owner](../../user-stories/gm-assign-token-owner.md)
- [room-member-view-connected-players](../../user-stories/room-member-view-connected-players.md)
