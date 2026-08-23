---
title: Room Member uses tabbed chat/initiative panel
created: 2026-07-29
status: done
---

As a Room Member
I want chat and the initiative tracker in a single tabbed panel
So that I can switch between them without the screen being cluttered with multiple separate panels

## Acceptance criteria

- [ ] The panel has a Chat tab and an Initiative Tracker tab
- [ ] Switching tabs doesn't lose an in-progress chat draft or tracker state
- [ ] This applies on both the desktop sidebar and the mobile bottom-sheet

## Verified

All three hold, with one reading worth naming: **the switch is a row of icon buttons at the foot
of the rail, not a tab widget.** Read literally, "a Chat tab and an Initiative Tracker tab" isn't
what shipped; read as what the story is for — one panel holding both, switched between, instead of
two panels competing for the rail — it is exactly what shipped, and the third icon (the room menu)
joined the same row. Taking the second reading, since the "So that" clause is about clutter rather
than about tabs.

The second criterion is the one that could have quietly failed and didn't. Both panels stay
mounted and are hidden with a CSS class rather than swapped with `{#if}`
(`web/src/routes/r/[slug]/+page.svelte`), so a half-typed message and the log's scroll position
both survive a trip to the tracker. There's a comment there saying so — worth leaving alone, since
an `{#if}` would look tidier and would silently break this.

Both layouts carry the same pair of hidden divs: the desktop `<aside>` rail, and the bottom sheet
below `lg`.
