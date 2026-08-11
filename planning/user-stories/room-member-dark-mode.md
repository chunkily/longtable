---
title: Dark mode for the app UI
created: 2026-08-05
status: done
---

As a Room Member
I want Longtable's interface to default to my system's color scheme, with the option to override it myself
So that the app matches my OS/browser theme out of the box, but I'm not stuck with a choice I dislike or that reverts every time my OS setting changes

## Acceptance criteria

- [x] The app UI (room page chrome, toolbar, chat panel, assets page, join/create room forms) follows the browser's `prefers-color-scheme` by default when I have no stored preference
- [x] I can override the theme (light / dark / system) from the room menu or the home page; my choice is persisted to client storage and wins over `prefers-color-scheme` from then on, on that device
- [x] The canvas/stage background (the area outside the map image) also switches, not just surrounding UI
- [x] The map image itself, uploaded token art, and the existing light/dark drawing-stroke palettes (see [dark-map-stroke-palette](../backlog/dark-map-stroke-palette.md)) are unaffected — this is app chrome, not map content
- [x] While following system default (no override stored), changing the OS/browser preference updates the app live, without a reload
- [x] Text and interactive elements meet reasonable contrast in both schemes (no light-gray-on-white or dark-gray-on-black leftovers from the light palette)

The second criterion said **from the options page** until it shipped. That page was never built:
one control doesn't justify a route, and from a full-bleed room it would have needed an entry in
the room menu to be reachable at all — so the control went in the menu instead of a link to it.
[options-page](../backlog/options-page.md) records what's left of that idea.
