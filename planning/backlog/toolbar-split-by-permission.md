---
title: Toolbar grouped by who can use it
created: 2026-08-14
status: done
tags: [ui, canvas]
---

The tool row (`map-toolbar.svelte`) groups by *gesture family* — hand, draw, measure, fog, ping
(`web/src/lib/tool-family.ts`) — which is the right split for the contextual strip beneath it, but
reads as arbitrary from a permissions angle: Fog is hidden outright for a Player
(`families = FAMILIES.filter((f) => f.value !== 'fog' || isGM)`, `map-toolbar.svelte:65`) while
everything else in the same row is available to everyone, with nothing marking the difference. New
token already sits in its own bordered sub-group (`map-toolbar.svelte:82-88`), separated by a
divider that has nothing to do with permission — it's just always been there.

Reorder the single row into two visually-divided clusters instead: everything a Player can use
(hand, draw, measure, ping, **and** New token — folded into this group, since it's available to
everyone and its existing divider wasn't marking anything) on the left, and GM-only tools (fog
today, room for more later) on the right of a divider — the same divider treatment New token
already has, repurposed to actually mean something. No permission logic changes: `requireGM` on
the server and the `isGM` filter on the client stay exactly as they are. This is purely a
rearrangement of `FAMILIES` and where `newToken` renders in `map-toolbar.svelte`.

## What shipped

`FAMILIES` in `map-toolbar.svelte` now lists fog last rather than between measure and ping, and the
single bordered row renders it past a second divider, gated on `isGM`, following the one that
already sat before New token. A Player still sees hand, draw, measure, ping and New token in one
undivided cluster — the divider (and Fog itself) simply isn't rendered — while a GM sees the same
cluster plus `| Fog` after a second divider. The two family-button blocks (Player's `{#each}` and
the GM's) shared identical markup, so that's a `{#snippet familyButton(family)}` now rather than
duplicated twice. No aria-labels changed and no permission logic moved — `requireGM` server-side
and the `isGM` client filter are untouched — so this is layout only, and the e2e specs that select
tools by label (`selectToolFamily(page, 'Fog')` etc.) didn't need touching. Verified with the built
binary (`web/e2e/run-app.mjs`'s server) as both a GM and a fresh Player seat: the DOM order and
divider placement match the design in both roles.
