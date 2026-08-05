---
title: Options page
created: 2026-08-05
tags: [ui, settings]
story: room-member-options-page
---

There's currently no place in the app for a Room Member to change a device-level setting after
the fact. Two other items now depend on this existing:

- [dark-mode](dark-mode.md) — needs a place to put a light/dark/system theme override
- [first-time-display-name-prompt](first-time-display-name-prompt.md) / the "I can change my
  display name later" criterion on
  [room-member-reusable-display-name](../../user-stories/room-member-reusable-display-name.md) —
  the display name is set once on first open with nowhere to revisit it since

Both settings are per-device, stored in `localStorage`, same as the display name already is —
this page is a UI for editing values that (mostly) already have a storage story, not a new
persistence mechanism.

Not yet decided: where the page lives in the nav/routing, and whether it's a modal/drawer or a
full route. Whoever picks this up should mock up the entry point and layout before wiring
anything — see the dark-mode-palette precedent for that (mocked options were discussed before
committing to hex values).

- [ ] Reachable entry point (toolbar icon / link, most likely — not tied to a specific room)
- [ ] Display name field, reads/writes the existing `localStorage` value
- [ ] Theme control (system / light / dark), reads/writes a new `localStorage` value, used by [dark-mode](dark-mode.md)
- [ ] Changes apply live, no reload

## Related user stories

- [room-member-options-page](../../user-stories/room-member-options-page.md)
