---
title: Dark mode for the app UI
created: 2026-08-05
---

As a Room Member
I want Longtable's interface to default to my system's color scheme, with the option to override it myself
So that the app matches my OS/browser theme out of the box, but I'm not stuck with a choice I dislike or that reverts every time my OS setting changes

## Acceptance criteria

- [ ] The app UI (room page chrome, toolbar, chat panel, assets page, join/create room forms) follows the browser's `prefers-color-scheme` by default when I have no stored preference
- [ ] I can override the theme (light / dark / system) from the [options page](../backlog/open/options-page.md); my choice is persisted to client storage and wins over `prefers-color-scheme` from then on, on that device
- [ ] The canvas/stage background (the area outside the map image) also switches, not just surrounding UI
- [ ] The map image itself, uploaded token art, and the existing light/dark drawing-stroke palettes (see [dark-map-stroke-palette](../backlog/open/dark-map-stroke-palette.md)) are unaffected — this is app chrome, not map content
- [ ] While following system default (no override stored), changing the OS/browser preference updates the app live, without a reload
- [ ] Text and interactive elements meet reasonable contrast in both schemes (no light-gray-on-white or dark-gray-on-black leftovers from the light palette)
