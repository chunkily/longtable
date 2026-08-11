---
title: Dark mode for the app UI
created: 2026-08-05
status: done
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

- [x] Audit hardcoded colors across room page, assets page, and join/create forms; move them to CSS custom properties
- [x] Add a dark palette via `prefers-color-scheme: dark`, checked for contrast
- [x] Canvas/stage background follows the scheme too
- [x] Theme control (light / dark / system) on the options page, persisted to `localStorage`, overriding system default when set
- [x] Verify live switching: OS theme toggled mid-session (no override stored) updates with no reload; a stored override persists across reloads and wins over the OS setting

## What shipped

**Most of the work described above was already done and never turned on.** `web/src/routes/layout.css`
has carried a full `.dark` palette since shadcn-svelte generated it, amber accent and all, and
`mode-watcher` has been a dependency the whole time — `ui/sonner/sonner.svelte` already imports
`mode` from it to theme toasts. Nothing put the `dark` class on `<html>`, so the second half of
that stylesheet was dead. The audit the first checkbox asks for found the app chrome already on
semantic tokens throughout; the only hardcoded colours left outside the canvas are a
`border-amber-500` callout on the assets page and a `bg-black/10` dialog scrim, both of which read
in either scheme.

So what actually shipped:

- **`<ModeWatcher>` in `+layout.svelte`**, with `modeStorageKey="longtable:theme"` so this
  browser's keys are all `longtable:`. The library migrates its old default key and deletes it, so
  nobody loses a choice.
- **A boot script in `web/src/app.html`**, which is the one piece that couldn't be bought off the
  shelf. mode-watcher normally prevents the flash of light with a `<svelte:head>` script, and that
  does nothing here: `ssr = false` means nothing a component renders reaches the HTML the browser
  is served, and a script inserted through `{@html}` never executes anyway. Head-script injection
  is turned off and app.html carries a real one. Confirmed present in `web/build/index.html`.
- **`ThemeToggle`** (`$lib/components/theme-toggle.svelte`): System / Light / Dark, bound to
  `userPrefersMode` rather than the resolved `mode`, so "following the device" stays visible as
  its own answer.
- **Two canvas colours**, in `game-canvas.svelte`. The grid was `#00000022`, invisible on a dark
  background, and the no-map slab was zinc-200, a lit rectangle in a dark room. Both are now
  `{ light, dark }` pairs. The broken-image slab moved to mid-grey: it used to be the *exact*
  value the dark placeholder now uses, which would have made "no map" and "map failed to load"
  identical.

### The options page didn't happen, on purpose

The item above says this depends on [options-page](options-page.md), and that page still doesn't
exist. Its own note anticipated this: *"If dark mode ends up shipping a plain three-way control
somewhere in the room chrome, this item may have nothing left in it."* That's what happened. The
control sits in the room menu — beside Leave room, which is the other thing there that changes
only this browser — and on the home page's welcome step. A route holding one control would be a
page that lies about how much is on it, and from a full-bleed room it would need an entry in that
same menu to be reachable at all, so the page was a hop rather than a home.

Not reachable from: the pre-join screen, and the assets page. Both are passed through rather than
sat in, and both are one step from somewhere that has it. If a second device setting ever turns
up, that's the moment to revisit the page.

### Reactivity trap, the same one as `resetView`

`mode.current` is read in **one** effect, which assigns a plain `let stageScheme` and calls
`render()`. The render functions read the variable. Reading `mode.current` inside them instead
would give every effect that calls a render function a dependency on the theme — the identical
shape to the bug recorded in [pinch-zoom-touch-devices](pinch-zoom-touch-devices.md), where a read
on the way through `applyViewChange` made clicking a token rebuild the map and broke selection
across 26 specs. `canvas.md` has the full note.

`stageScheme` is seeded from the current scheme rather than defaulting to light. Defaulting meant
a dark browser painted light, then re-rendered immediately — two `renderMap` calls with two loads
of the same image in flight, and whichever finished last winning.

### Tests

`web/e2e/theme.spec.ts`, five tests, all confirmed failing against the unfixed code. The one worth
keeping honest is the first: it blocks `_app/immutable/entry/*.js` so the app never hydrates, which
is the only way to observe the boot script doing its job — every other test passes without it,
because mode-watcher sets the class a moment later and the bug is a white flash in between.

The map assertions read pixel colour off the grid layer rather than counting opaque pixels the way
the other specs do. Counting would pass in both schemes: the lines are there either way, and what
breaks in the dark is that they're black.

## Related user stories

- [room-member-dark-mode](../user-stories/room-member-dark-mode.md)
