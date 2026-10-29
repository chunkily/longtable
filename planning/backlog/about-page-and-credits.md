---
title: An About page, with credits by artist
created: 2026-10-29
status: open
tags: [licensing, assets, docs]
---

The game-icons.net credit under the icon grid says "Some icons by Lorc, Delapouite and others",
because the `@iconify-json/game-icons` package dropped who drew each icon. CC BY 3.0 asks for the
creator's name, and "and others" is a guess standing in for data we could have.
[game-icons-second-icon-pool](game-icons-second-icon-pool.md) has the background.

Two parts, and the first is worth doing even if the second waits.

**Per-icon artists.** game-icons.net files every icon under its artist —
`game-icons.net/1x1/lorc/dragon-head.html` is Lorc's — and the `game-icons/icons` GitHub repo is
laid out the same way, one folder per artist. `web/scripts/vendor-game-icons.mjs` would read that
layout (the file listing alone is enough, through the GitHub API; the SVGs already come from the
npm package) and record an `author` on every generated entry. Then:

- an icon's tooltip in the picker names its artist ("Dragon, by Lorc");
- the credit under the grid stops guessing — "Icons from game-icons.net" with the artist list a
  link away;
- the About page can list each artist with the icons used from them.

Nothing from the repo ships except the names. It's a vendor-time input like the npm package, and
needs the same "copied, not imported" treatment.

**An About page.** A plain `/about` route — something read and left, so a page rather than a
dialog — linked from a small footer on the home page and from the room menu beside Leave room,
where a Player can reach it too. It holds:

- what Longtable is, its version, and its own licence;
- third-party credits: Lucide (ISC), game-icons.net (CC BY 3.0, by artist), Inter (OFL), Konva
  and Svelte (MIT) — the notices the first release has to carry anyway, since the binary ships
  all of them;
- a link to the repository.

The version should come from the binary (`GET /api/health` could carry it), so the page and
`longtable -h` can't disagree. `check-for-new-versions` would want the same number.

CLAUDE.md records dropping an options page because "one control doesn't earn a route". This one
has real content, so it doesn't fall foul of that — but it shouldn't grow settings either.

- [ ] Script records each game-icon's artist from the `game-icons/icons` listing
- [ ] Picker tooltip names the artist; the grid's credit links to the full list
- [ ] `/about` with version, licence, third-party credits and the repo link
- [ ] Linked from the home page and the room menu
