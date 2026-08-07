---
title: Branding (favicon + title)
created: 2026-07-29
status: done
tags: [branding, ui]
story: room-member-sees-branding
---

Add branding in the form of a favicon and a stylized title.

## What shipped

A real favicon — a small amber tabletop glyph in `web/src/lib/assets/favicon.svg` — replaced the
SvelteKit starter's Svelte-logo placeholder that every page had been serving since the project was
scaffolded. It's referenced from the root layout, so it's still the one favicon for every page.

The root layout now sets a default `<title>Longtable</title>`. A route with its own
`<svelte:head><title>` replaces that default rather than sitting next to it — verified directly in
a browser rather than assumed, since the more obvious guess (multiple `<title>` elements land in
the DOM and the first one in tree order wins, which would make a layout default un-overridable)
turns out to be wrong for Svelte's head handling. That makes the layout title a real fallback: the
home page needs no title of its own, while the room and assets pages set one following a
most-specific-first chain that ends in the brand name — `{roomName} — Longtable` in a room, `Assets
— {roomName} — Longtable` on the assets page. The room title falls back to `seatsRoomName` (fetched
for the pre-join seat picker) and then the slug, so the tab reads the room's actual name even
before a device has joined and gotten a `Session` back — the same fallback chain the pre-join card
itself already used for its heading.
