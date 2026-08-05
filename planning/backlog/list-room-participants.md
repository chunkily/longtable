---
title: List a room's participants
created: 2026-07-30
status: done
tags: [rooms, data-model]
---

Add a way to list everyone who has ever joined a room — `SELECT * FROM participant WHERE
room_id = ?`, exposed over the WS protocol (likely folded into `state.sync`, the way tokens and
drawings already are). Nothing in the store or protocol currently does this at all: connectivity
is tracked only in-memory, in `Hub.rooms[roomID]`, which knows who's connected right now but
nothing about the room's roster as a whole.

This is now the **only** thing standing between
[token-detail-panel](token-detail-panel.md) and `gm-assign-token-owner`: the panel
shipped with name, art, size and visibility, and left owner out solely because nothing can say
who the candidates are. `token.update` already loads the token and edits it in place, so adding
an owner field to it is a line in the request struct and a line in the assignment block once
there's a list to pick from.

Found while scoping [token-size-and-owner-pickers](token-size-and-owner-pickers.md):
an owner picker needs to offer every Room Member who could plausibly own a token, including a
Player who joined last week and isn't online at the moment a GM is prepping tokens before a
session — that's a different list from "who's connected right now."

Keep this distinct from [connected-players-list](connected-players-list.md), which wants the
live subset. That feature will likely still need this roster as its base, with the
currently-connected ones (from `Hub.rooms`) highlighted or filtered on top of it — but the two
have different sources of truth and shouldn't be conflated into one query.

## Related user stories

- [gm-assign-token-owner](../user-stories/gm-assign-token-owner.md)
- [room-member-view-connected-players](../user-stories/room-member-view-connected-players.md)

## What shipped

`ListParticipantsForRoom` plus a `participants` array in `state.sync`, alongside a separate
`connectedParticipantIds` — shipped together with
[broadcast-participant-presence](broadcast-participant-presence.md) and
[connected-players-list](connected-players-list.md), since none of the three is visible alone.

**The roster query deliberately does not select `session_token`.** It is a credential, this list
is the basis of something sent to every client in the room, and the surest way for a payload
builder never to leak it is for it never to be loaded — `store.Participant.SessionToken` comes
back zero from this query. `participantPayload` is a second line of defence: an exhaustive
struct-to-map rather than marshalling the struct, so a field added to the model can't silently
start being broadcast. Two tests guard it, one checking the field and one grepping the whole
`state.sync` payload for the GM's actual token.

The two lists are kept apart on the wire exactly as this item argued they should be. The owner
picker still doesn't exist — see
[token-size-and-owner-pickers](token-size-and-owner-pickers.md) — but nothing is
blocking it now.
