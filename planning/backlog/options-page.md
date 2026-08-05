---
title: Options page
created: 2026-08-05
status: open
tags: [ui, settings]
story: room-member-options-page
---

There's currently no place in the app for a Room Member to change a device-level setting after
the fact. [dark-mode](dark-mode.md) needs one — somewhere to put a light/dark/system override.

**This item was half about display name until 2026-08-05**, when that was dropped along with
[room-member-reusable-display-name](../user-stories/room-member-reusable-display-name.md): seats
mean a name is typed once per room, so there is no device-level name to edit. What's left is the
theme control, which is enough to justify the page but not much more — so it's worth asking
whether this should be a page at all, or a menu item next to the theme toggle it exists to hold.
If dark mode ends up shipping a plain three-way control somewhere in the room chrome, this item
may have nothing left in it.

Not yet decided: where the page lives in the nav/routing, and whether it's a modal/drawer or a
full route. Whoever picks this up should mock up the entry point and layout before wiring
anything — see the dark-mode-palette precedent for that (mocked options were discussed before
committing to hex values).

- [ ] Reachable entry point (toolbar icon / link, most likely — not tied to a specific room)
- [ ] Theme control (system / light / dark), reads/writes a `localStorage` value, used by [dark-mode](dark-mode.md)
- [ ] Changes apply live, no reload

## Related user stories

- [room-member-options-page](../user-stories/room-member-options-page.md)
