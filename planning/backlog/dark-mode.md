---
title: Dark mode for the app UI
created: 2026-08-05
status: open
tags: [ui]
story: room-member-dark-mode
---

The app UI is light-only today. Add a dark color scheme that defaults to `prefers-color-scheme`
when the user hasn't chosen one, with a manual light/dark/system override stored in client
storage (`localStorage`) once they do. The override lives on the
[options page](options-page.md), which doesn't exist yet — this item depends on that page
existing, at least as a stub with a theme control on it.

Scope is app chrome — room page (toolbar, chat panel, connection status), assets page, and the
join/create room forms — plus the canvas/stage background behind the map. It does not touch map
images, token art, or the drawing-stroke color palettes: those are map content, and the dark
stroke row already has its own item ([dark-map-stroke-palette](dark-map-stroke-palette.md)) for
a different reason (legibility against a dark map, not app theming).

Likely means introducing CSS custom properties for the colors currently hardcoded in component
styles, with a `@media (prefers-color-scheme: dark)` block for the no-override default, and a
stored preference (e.g. a `data-theme` attribute set from `localStorage` on load) taking priority
over the media query when present. While following system default, the switch should apply live
if the OS setting changes mid-session, not just on load.

- [ ] Audit hardcoded colors across room page, assets page, and join/create forms; move them to CSS custom properties
- [ ] Add a dark palette via `prefers-color-scheme: dark`, checked for contrast
- [ ] Canvas/stage background follows the scheme too
- [ ] Theme control (light / dark / system) on the options page, persisted to `localStorage`, overriding system default when set
- [ ] Verify live switching: OS theme toggled mid-session (no override stored) updates with no reload; a stored override persists across reloads and wins over the OS setting

## Related user stories

- [room-member-dark-mode](../user-stories/room-member-dark-mode.md)
