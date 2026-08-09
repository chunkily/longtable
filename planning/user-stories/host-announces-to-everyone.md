---
title: Host announces something to everyone on the server
created: 2026-08-09
status: done
---

As a Host
I want to set a message that everyone on my server sees until they dismiss it
So that I can tell every table about a restart, a move or an outage without knowing who they are or being at any of their tables

## Acceptance criteria

- [ ] I can set the message without being in a room, and without a GM password for any of them
- [ ] It is shown to everyone on the server — in a room or not, GM or Player
- [ ] Anyone can dismiss it, and it stays dismissed for them
- [ ] Changing the message brings it back for people who dismissed the previous one
- [ ] A server with no message set shows nothing, and nothing about the page changes
- [ ] It doesn't cover anything — the map and its toolbar move down rather than sliding underneath

## Verified 2026-08-09

All six hold. Set with `longtable serve -banner "…"` — a flag rather than a screen because a Host
runs the server and needn't be at any table on it, so there is nowhere in the web UI that is
theirs (see [roles.md](../roles.md), and
[host-restores-room-access](host-restores-room-access.md) for the same reasoning about the
recovery path).

The fourth criterion is the one that shapes the storage: dismissal is remembered as *the message
itself*, not a flag, so a Host who changes the text is saying something new and it reaches
everyone again. The sixth is the one that needed real work — see the backlog item.
