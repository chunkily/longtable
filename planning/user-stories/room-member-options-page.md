---
title: Room Member changes persistent settings
created: 2026-08-05
---

As a Room Member
I want a settings page where I can change my display name and theme preference
So that I can adjust device-level preferences without a first-time prompt being my only chance to set them

## Acceptance criteria

- [ ] Reachable from the app (entry point TBD — likely the room toolbar or a global nav element, since it isn't tied to any one room)
- [ ] Lets me change my display name; persisted to client storage and used as the default for future room joins/creates (this is the "change it later" half of [room-member-reusable-display-name](room-member-reusable-display-name.md))
- [ ] Lets me override the color theme (system / light / dark); persisted to client storage
- [ ] Changes take effect immediately, without a reload
