---
title: Host sees LAN-reachable URLs on startup
created: 2026-07-29
status: incomplete
---

As a Host
I want the server to print the addresses players on my network can use to connect, when it starts up
So that I don't have to hunt through OS network settings to figure out what to share

## Acceptance criteria

- [ ] On startup, the server logs one or more LAN-reachable URLs (e.g. `http://192.168.x.x:8080`) alongside the existing listening address
- [ ] If the machine has multiple network interfaces, all reasonable candidates are shown rather than guessing which one is correct
- [ ] This requires no configuration — it's automatic, based on the machine's current network interfaces
