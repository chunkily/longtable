---
title: Room Member changes persistent settings
created: 2026-08-05
status: incomplete
---

As a Room Member
I want somewhere to change the settings that belong to this device rather than to a room
So that a preference I set once stays set, without living inside a room I might leave

## Acceptance criteria

- [ ] Reachable from the app (entry point TBD — likely the room toolbar or a global nav element, since it isn't tied to any one room)
- [ ] Lets me override the color theme (system / light / dark); persisted to client storage
- [ ] Changes take effect immediately, without a reload

The display-name criterion was removed on 2026-08-05 when
[room-member-reusable-display-name](room-member-reusable-display-name.md) was dropped — with
seats, a name is typed once per room and there is no device-level one to edit. That leaves the
theme as the only thing this page holds, so it's fair to ask whether it earns being a page; see
[options-page](../backlog/options-page.md).
