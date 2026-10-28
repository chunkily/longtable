---
title: Token art keeps its proportions on the map
created: 2026-10-06
status: done
tags: [tokens, canvas, bug, playtest]
story: room-member-frames-token-art
---

Playtest feedback: portraits look squashed on the map. The cause is `addTokenImage` in
`game-canvas.svelte`, which draws the art at exactly the token's width and height and rounds the
corners into a circle — every token is square (Tiny through Gargantuan are all N×N), so any art
that isn't is scaled unevenly to fit. A 2:3 portrait renders at two-thirds of its real height.

The same art is drawn three different ways today, and nobody chose any of them:

| Where | How |
| --- | --- |
| The map | stretched to the token's box |
| Initiative panel | `object-cover` — a centred circular crop, proportions intact |
| Library and pickers | `object-contain` — the whole image, letterboxed |

- [ ] Token art on the map keeps its aspect ratio, covering the circle and centred — the same
      framing the initiative panel already uses
- [ ] Holds at every token size and zoom
- [ ] Art already in a library and tokens already on a map are fixed by this alone, with nothing
      re-uploaded

This lands first and on its own. It is also the permanent fallback for non-square token art that
never passes through the cropper in [frame-token-art-at-upload](frame-token-art-at-upload.md) —
everything uploaded before that ships, and anything that reaches the upload endpoint some other
way — so it stays correct after the cropper exists rather than being superseded by it.

In Konva the cover maths is a `crop` on the `Konva.Image` (the largest centred square of the
source) rather than a different width and height, which keeps the circle exactly the token's size.

## What shipped

Token art on the map is cropped to the token's shape rather than stretched to it: the largest
centred region of the art that fits, scaled evenly — the same framing the initiative panel's
`object-cover` already gave it. Existing tokens and library art are fixed by it with nothing
re-uploaded.

The arithmetic is `web/src/lib/cover-crop.ts`, tested on its own, and `addTokenImage` passes its
result as the `Konva.Image`'s `crop` in the image's natural pixels. Checked on the canvas with a
300×100 image in three coloured bands: the token shows the green middle band alone, where the
stretched version showed all three. The repo's own `wide-map.png` couldn't show the difference —
it's one flat colour, so cropped and stretched look identical — which is worth knowing before
reaching for it to test anything about framing.

The library tile still shows token art whole (`object-contain`), on purpose: it's where you see
everything you uploaded before choosing it. Once [frame-token-art-at-upload](frame-token-art-at-upload.md)
lands, new token art arrives square and the three framings agree anyway.
