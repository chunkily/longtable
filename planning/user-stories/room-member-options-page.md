---
title: Room Member changes persistent settings
created: 2026-08-05
status: done
---

As a Room Member
I want somewhere to change the settings that belong to this device rather than to a room
So that a preference I set once stays set, without living inside a room I might leave

## Acceptance criteria

- [x] Reachable from the app: the room menu, and the home page's welcome step
- [x] Lets me override the color theme (system / light / dark); persisted to client storage
- [x] Changes take effect immediately, without a reload

The display-name criterion was removed on 2026-08-05 when
[room-member-reusable-display-name](room-member-reusable-display-name.md) was dropped — with
seats, a name is typed once per room and there is no device-level one to edit. That leaves the
theme as the only thing this page holds, so it's fair to ask whether it earns being a page; see
[options-page](../backlog/options-page.md).

**It didn't, and "somewhere" turned out not to be a page.** Every criterion above is met by the
theme control that shipped with [dark-mode](../backlog/dark-mode.md) — it is reachable, it
persists to this device, and it applies live. The story asked for somewhere to change a device
setting, not for a route, and the room menu is somewhere: it already holds Leave room, which is
the other thing there that changes only this browser. The backlog item is
[dropped](../backlog/options-page.md), with a note on what would justify reopening it.
