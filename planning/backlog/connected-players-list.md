---
title: Connected players list
created: 2026-07-29
status: done
tags: [chat, ui]
story: room-member-view-connected-players
---

Display all connected players in the chat area.

## What shipped

A row of name badges at the top of the panel — desktop sidebar and mobile sheet both, one snippet
rendered twice — showing who is connected right now, with the GM marked. It updates live from
[broadcast-participant-presence](broadcast-participant-presence.md) and starts from the
`connectedParticipantIds` in `state.sync`.

Only the connected are listed, per the story's third criterion: someone who joined last week and
isn't online doesn't appear, even though `client.participants` is holding them for the owner
picker to come. `RoomClient.connectedParticipants` is the derived intersection.

One criterion is satisfied only trivially and should be re-checked when tabs land: "the list is
visible regardless of which tab is active" — there are no tabs yet, so it sits above the token
details and the chat log. When [chat-panel-tabs](chat-panel-tabs.md) ships, this
row needs to stay *outside* the tab switcher rather than becoming part of the chat tab.
