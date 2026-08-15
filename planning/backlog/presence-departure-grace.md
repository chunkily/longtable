---
title: Server-authoritative presence, with a grace period on leaving
created: 2026-08-15
status: done
tags: [presence, ws]
story: room-member-stable-presence-list
---

The hub announces `participant.disconnected` the instant a participant's last connection closes,
and the client drops them from `connectedParticipantIds` on receipt. Nothing debounces it, so a
blip takes the badge off everyone else's rail and the reconnect puts it back — 500ms at best
(`RECONNECT_BASE_MS`), and up to about a minute as the backoff doubles towards its 15s ceiling.
A phone locking its screen does it. A server restart does it to everyone at once.

Presence today is derived from the socket map and nothing else, which is why it can't answer "are
they gone, or are they coming straight back?" — the only question anyone at the table is actually
asking.

Make the hub the authority instead:

- [ ] A participant whose last connection closes is **still present** for a grace period rather
      than immediately absent.
- [ ] Reconnecting inside that window is a resumption: the timer is cancelled and nothing is
      broadcast at all, because nobody was ever told they left.
- [ ] Once the window expires, `participant.disconnected` goes out as it does today.
- [ ] `ConnectedParticipantIDs` counts anyone inside their grace window as connected, so a client
      that syncs mid-window sees the same room as everyone else. Getting this wrong is worse than
      the flicker: the arrival that would have corrected it never fires, because a resumption is
      deliberately silent.

This is also the foundation for [chat-log-timestamps-and-events](chat-log-timestamps-and-events.md)
— a durable "left the room" line hung off an undebounced disconnect would write permanent noise
every time a connection blipped, and unlike a badge, a chat line doesn't heal itself.

## Related user stories

- [room-member-stable-presence-list](../user-stories/room-member-stable-presence-list.md)

## What shipped

The hub keeps a presence model instead of reading the socket map: `unregister` starts a
`time.AfterFunc` and announces nothing, `register` cancels it and — this is the point — announces
nothing either, because the room was never told they left. Only an expired timer produces
`participant.disconnected`. `-departure-grace` sets the window, 30s by default.

Decisions worth not rediscovering:

- **`ConnectedParticipantIDs` counts anyone mid-grace as connected**, and the checkbox above
  calling that worse than the flicker was right. A resumption is silent, so a client that synced
  during someone's window would never receive the arrival that fixed it — the person would simply
  be missing from that one rail until something else resynced.
- **`finishDeparture` re-checks the pending entry under the lock.** `time.AfterFunc` can't un-fire:
  a timer stopped a microsecond too late is already running while `register` holds the lock, and
  the entry's absence is what tells it to stand down.
- **The room's entry in the socket map outlives its last connection**, until the departing set is
  empty too. Dropping it on the last disconnect would take the pending timer's room with it, which
  is exactly the case that has to keep working.
- **A server restart writes no departures** — the timers die with the process — and then everyone's
  reconnect posts a fresh `joined`. Rare, honest, and cheaper than persisting pending departures.
- The e2e harness runs with `-departure-grace 2s`: long enough that a spec reloading a page doesn't
  trip a departure, short enough for a presence assertion to outlast it. Six specs failed the first
  time this shipped for exactly that reason.
