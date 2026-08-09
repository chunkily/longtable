---
title: Host sees LAN-reachable URLs on startup
created: 2026-07-29
status: done
---

As a Host
I want the server to print the addresses players on my network can use to connect, when it starts up
So that I don't have to hunt through OS network settings to figure out what to share

## Acceptance criteria

- [ ] On startup, the server logs one or more LAN-reachable URLs (e.g. `http://192.168.x.x:8080`) alongside the existing listening address
- [ ] If the machine has multiple network interfaces, all reasonable candidates are shown rather than guessing which one is correct
- [ ] This requires no configuration — it's automatic, based on the machine's current network interfaces

## Verified 2026-08-09

All three hold, against `internal/lanurl` and the four lines of wiring in `serve`. Run on this
machine, a wildcard bind prints the Ethernet address first and the two Hyper-V virtual switches
after it, each with its interface name — the second criterion working exactly as written, since
two of those three are the wrong answer and only the Host knows which.

One case the criteria don't mention and the implementation has to get right: **a server started on
a specific `-addr` is answered with that address alone.** Bound to `127.0.0.1:8080` it is reachable
there and nowhere else, so enumerating the machine's interfaces would print a confident lie —
which is worse than printing nothing, because it looks like the answer.

See [startup-prints-lan-urls](../backlog/startup-prints-lan-urls.md) for the rest of the reasoning.
