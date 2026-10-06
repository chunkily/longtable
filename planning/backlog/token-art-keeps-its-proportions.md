---
title: Token art keeps its proportions on the map
created: 2026-10-06
status: open
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
