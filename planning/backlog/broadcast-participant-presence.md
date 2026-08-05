---
title: Broadcast connected participant presence
created: 2026-07-29
status: done
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

## What shipped

`participant.connected` / `participant.disconnected`, from `register`/`unregister`, with the live
set also in `state.sync` as `connectedParticipantIds`. Shipped with
[list-room-participants](list-room-participants.md) and
[connected-players-list](connected-players-list.md).

Three decisions worth keeping:

- **A person is not a connection.** `register` and `unregister` now return whether this was the
  *first* or *last* connection that participant had open, and only those broadcast. Two browser
  tabs are two clients and one person at the table; without this, opening a tab announces someone
  who was already there and closing it announces them gone while they're still looking at the map.
- **`participant.connected` skips its own sender** — the only broadcast in the protocol that
  does. Every other one echoes back because the sender has an optimistic render to reconcile;
  here the arriving client's `state.sync` already lists it as connected. This also spared every
  existing multi-client test from having to read past an event about itself.
- **Arrival carries the whole participant, departure only an id.** A first-time joiner is on
  nobody else's roster yet, so arrival upserts; departure leaves the roster alone, because
  leaving the table isn't leaving the room.

`announceDeparture` needs a fresh context, like `endMeasurementOnDisconnect` beside it — the
request context is already cancelled once the connection has dropped, so writing on it goes
nowhere. It is also declared *before* the unregister defer so it runs after it (LIFO), which is
what keeps the leaver out of the recipients of its own departure.

**This changed the test harness, and that is the part most likely to surprise.** Adding any
broadcast that fires on connect shifts every already-connected client's stream by one, and 32
existing tests failed on it. Rather than teach each of them about presence, `readEnvelope` now
skips presence events and `readAnyEnvelope`/`readPresence` are the unfiltered and inverse
readers. See `references/testing.md`, which also records that `expectNoMessage` leaves its
connection unusable — a fact this item ran into and that had been true all along.
