---
title: Broadcast connected participant presence
created: 2026-07-29
tags: [chat, data-model, websocket]
story: room-member-view-connected-players
---

Expose live connection state to clients. Today `Hub.rooms map[string]map[*client]struct{}`
(`internal/ws/hub.go:39`) tracks who's currently connected in-memory, but `register`/`unregister`
(lines 105-121) never broadcast anything — no client ever learns who else is online. This is also
distinct from `store/participant.go`'s permanent roster, which tracks everyone who has ever
joined, not who's connected right now.

Needs a new broadcast (e.g. on register/unregister) and a client-side message type so Room
Members can see a live connected/disconnected list.
