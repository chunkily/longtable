---
title: GM fog view doesn't read clearly on some maps
created: 2026-08-11
status: done
tags: [fog, canvas]
---

`renderFog` in `web/src/lib/components/game-canvas.svelte` gives the GM a black cover at 0.35
opacity over hidden cells, and punches revealed cells out of it with a *second* black rect at 0.35
opacity and `globalCompositeOperation: 'destination-out'`. That composite operation multiplies the
existing alpha down rather than clearing it, so a revealed cell ends up at roughly
0.35 × (1 − 0.35) ≈ 0.23 opacity — not 0. The GM view is really showing two shades of the same
dim tint rather than "fogged" vs. "clear", which reads fine on some map art and is nearly
impossible to tell apart on others (reported against a specific map in-session, exact one not
recorded).

- [ ] Revealed cells are fully clear for the GM (punch to 0 tint, not ~0.23)
- [ ] Hidden-cell opacity raised enough to stay clearly visible now that revealed cells are
      fully clear (0.35 read as too faint on its own; try 0.5)
- [ ] Player view is unaffected — players already get a full destination-out punch with no tint
      residue, this only touches the GM branch

Deliberately not doing yet, raised and set aside during planning:

- A non-black tint color or hatch pattern, so contrast doesn't depend on the map's own palette.
  Worth it only if the opacity fix alone still isn't enough on the map that prompted this.
- A per-room/per-scene adjustable opacity setting. Bigger scope (UI + persistence) than a fixed
  constant needs unless the flat fix proves insufficient across maps.

## What shipped

`renderFog` in `game-canvas.svelte` no longer gives the GM a second, weaker punch-out — revealed
cells are now fully cleared for both roles by one shared loop, and the GM's cover alone carries
the hidden/revealed contrast, raised from 0.35 to 0.5 opacity. Verified directly against rendered
canvas pixels (not just visually): the GM's fog layer now reads a clean 128-vs-0 alpha split
(≈0.5 opacity vs. fully clear) where it used to be roughly 89-vs-59 — two shades of the same dim
gray, which is exactly what made it unreadable on some map art.

Player view is untouched: it already did a full punch-out with an opaque cover, and
`fog-controls.spec.ts` (which asserts only against the Player's canvas) still passes unchanged —
its stale rationale comment, which cited the old 0.35 GM numbers, was corrected in the same commit.

Not done here: a non-black tint/pattern — still nobody has asked for one. A configurable opacity
*was* revisited, the same session this shipped in, once new scenes started defaulting to fully
revealed (see [fog-rect-painting-and-defaults](fog-rect-painting-and-defaults.md)) and a flat
constant stopped being enough for every map. The slider it shipped with reads from and writes to
this same `renderFog` opacity parameter — this item's 0.5 is now just that control's default.
