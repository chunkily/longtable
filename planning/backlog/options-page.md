---
title: Options page
created: 2026-08-05
status: dropped
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

## Dropped 2026-08-11

The last paragraph above called it: [dark-mode](dark-mode.md) shipped its control straight into
the room menu and the home page's welcome step, and there is nothing left for a page to hold.

The argument that settled it wasn't only "one setting is too few". The room page is full-bleed,
with no header and no nav — a `/options` route would have needed an entry in the room menu to be
reachable from the only screen anyone spends an evening on. So the menu was where it got reached
from either way, and the page was a hop rather than a home. It would also have needed a back link
that knew whether you'd arrived from a room or the home page, which is real machinery for one
three-way control.

**Reopen this if a second device-level setting turns up**, and the shape to reopen it as is
probably a dialog from the room menu rather than a route — that keeps the full-bleed layout intact
and doesn't ask where "back" goes. Candidates that would do it: a reduced-motion override, a
default zoom, a font size for the chat panel.

Two places still have no theme control: the pre-join screen and the assets page. Both are passed
through rather than sat in, and both are one step from somewhere that has one. Worth its own small
item if anyone finds themselves wanting it, not worth a page.

## Related user stories

- [room-member-options-page](../user-stories/room-member-options-page.md)
