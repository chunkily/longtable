---
title: Room Member drags the map with the right mouse button
created: 2026-08-14
status: done
---

As a Room Member
I want to drag the map with the right mouse button
So that I can move around the battle map without putting down the ruler, the pen or the fog brush
I am part-way through using

## Acceptance criteria

- [ ] A right-button drag on the map pans it, with any tool selected and with none
- [ ] A middle-button drag does the same
- [ ] The map follows the pointer exactly — it neither lags behind nor runs away, at any zoom level
- [ ] Panning starts anywhere on the map, including on top of a token, and never picks that token
      up or changes anything anyone else can see
- [ ] The browser's context menu never opens over the map
- [ ] I can pan part-way through a left-button gesture — holding the right button as well shoves
      the map along so a ruler, template or shape can be dragged past the edge of the screen
- [ ] Doing that leaves the gesture itself untouched: the far end stays where I left it while the
      map moves, nothing is committed early, and it still ends on its own left-button release
- [ ] The left button still does what it did — the active tool's gesture, or a pan in the Hand tool
- [ ] Touch input is unaffected: one finger still pans, two still pinch
