---
title: game-icons.net as a second icon pool
created: 2026-10-06
status: done
tags: [tokens, assets, licensing]
story: room-member-makes-a-token-without-art
---

Lucide is a general-purpose set and thin on monsters — no dragon, goblin, orc or beholder.
[game-icons.net](https://game-icons.net) has four thousand-odd RPG icons, and is the obvious second
pool once [icon-tokens](icon-tokens.md) exists. Agreed as "later" rather than now.

Two things make it more than a list of keys:

- **Licence.** CC BY 3.0 — attribution is required, and it has to appear somewhere a Room Member
  can actually see, not only in a file in the repo. Where that lives is the first decision this
  item has to make.
- **Vendoring.** The SVGs are copied into the repo (or a curated subset is), with a note on where
  they came from and how to refresh them. Four thousand icons is too many to show unfiltered; it is
  a curation job as much as a technical one, and search words matter more here than in the Lucide
  pool.

**Groundwork exists (2026-10-28):** `store.TokenIcons` and `TOKEN_ICONS` in
`web/src/lib/token-icons.ts` are the two lists to extend, and `token-icon-image.ts` turns an icon
component into something the canvas can draw — a vendored SVG could skip the mounting and go
straight to the data URL. The picker searches `words`, which matters far more at four thousand.

`icon-tokens` namespaces keys (`lucide:sword`) precisely so this is additive: a `game-icons:` prefix
and a second list, no migration, and every existing token unchanged.

## What shipped

(2026-10-29) The pool exists and is the GM's to fill: `web/src/lib/game-icons.picks.json` names
each icon (the last part of its game-icons.net address), its label and its search words, and
`npm run icons` in `web/` copies the picked ones out of the `@iconify-json/game-icons` dev
dependency into `game-icons.generated.ts` and `internal/store/token_icon_game_icons.go`.
`npm run icons -- --search wolf` lists names. It ships with 85 picked to fill what Lucide can't — monsters,
monstrous beasts and NPC types — and to skip what it already covers. A few are stand-ins where the
set has nothing exact (`evil-eyes` as Beholder, `crowned-skull` as Lich, `transparent-slime` as
Gelatinous cube), and kobold, gnoll, bugbear, owlbear and manticore have no icon at all, so their
names are search words on the nearest one instead.

- **Credit sits under the icon grid**, shown whenever a game-icons icon is among the matches
  (`needsGameIconsCredit`). The Iconify package carries no per-icon authors, so it names Lorc and
  Delapouite, who drew most of the set, "and others", with links to the site and the licence.
- **Copied, not imported.** The package is one 6 MB JSON file of every icon; importing it would
  ship all of them to every Room Member. The generated files are checked in so a build needs no
  script step, and `TestTokenIcons_MatchTheClientList` reads both client files, so a hand-edited
  generated file still fails it.
- **Filled, not stroked.** These are solid shapes in `currentColor`, which `iconSvg` replaces with
  the ink outright rather than leaving to a `color` attribute an image decoder may not honour.
  `token-glyph.svelte` draws either pool off the canvas.
- Tests take the *first* pick rather than naming one, so trimming the picks doesn't break them.
