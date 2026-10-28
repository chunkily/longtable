---
title: game-icons.net as a second icon pool
created: 2026-10-06
status: open
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
