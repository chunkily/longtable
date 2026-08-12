---
title: Fog paints by rectangle, scenes start revealed, opacity is a GM setting
created: 2026-08-11
status: done
tags: [fog, canvas]
story: gm-hide-fog-cell
---

Three related fog changes, decided together in one planning session and shipped together:

1. **Rectangle painting.** Fog reveal/hide painted per cell the pointer swept over — precise, but
   slow for a big room, and the tool this repo already has for "cover an area fast" is a
   rubber-band drag (`rect`/`line`/`ellipse` drawing already work this way). An L-shaped room is
   two drags, not a reason to keep a freeform sweep; true freeform polygons are out of scope, same
   as the original fog-of-war-controls item decided.
2. **New scenes start fully revealed, not fully covered.** A Player joining a scene nobody has
   painted fog on yet saw an unexplained black rectangle — indistinguishable from a broken map
   load rather than "nothing revealed". `ClearFog` ("reset fog") is still how a GM gets the
   classic fully-covered dungeon-crawl start; it just isn't the default anymore.
3. **Fog opacity is a GM-adjustable slider, not a fixed constant.** Follows directly from #2:
   once a flat opacity constant existed only to make *hidden* cells readable
   ([fog-gm-view-contrast](fog-gm-view-contrast.md)), a GM who deliberately turns fog on for a
   specific map needs to be able to tune how strongly it reads on that map, the same way the
   original bug report was about one specific map's contrast.

## What shipped

**Rectangle fog painting** — `game-canvas.svelte`'s fog handler now rubber-bands a rectangle from
the `mousedown` point to wherever the pointer is (the same shape `line`/`rect`/`ellipse` drawing
already use), snapped live to whole grid cells so the preview always shows exactly the set that
will be sent. On release, every cell in that bounding box goes out in one `fog.reveal`/`fog.hide`
command — a plain click with no drag degenerates to a 1×1 box, so a single-cell touch-up still
works. `paintAtPointer`, the `painting` flag and the `pendingCells` sweep map are gone along with
the old per-cell-crossed gesture.

One thing this made newly possible: a corner-to-corner drag across a large map can now name far
more cells in one command than a hand-swept drag ever could (a mouse sweep was self-limiting;
a rectangle isn't). `fog.reveal` and `fog.hide` now cap `len(cells)` at `maxRevealAllCells`
(40,000 — the same cap `fog.revealAll` already enforced) rather than trusting the client to stay
reasonable. (That constant is now `maxPaintedCells`, and `fog.revealAll` no longer has a cap of
its own — see the superseding item below.)

**Scenes start revealed** — `handleSceneCreate` now calls the same `sceneFogCells` enumeration
`fog.revealAll` uses and reveals every cell immediately after creating the scene, before the first
`scene.created` broadcast. Best-effort: a scene too large for that cap (same 40,000-cell ceiling)
just starts covered like every scene used to — scene creation itself must never fail over this.
`ClearFog`'s doc comment, which called fully-covered "the state a scene starts in", no longer
claims that.

> **Superseded**, 2026-08-12, by
> [fog-hidden-set-packed-chunks](fog-hidden-set-packed-chunks.md): fog now stores what's *hidden*,
> so a new scene comes up revealed by holding no rows at all and there is nothing to materialise,
> nothing to cap, and no map too large to get the default. The materialisation described above is
> gone; the behaviour it was reaching for is what the storage does on its own.

**Fog opacity slider** — new `$lib/fog-opacity.ts` (persisted to `localStorage` per browser, same
pattern as the theme control — it's how fog looks on *this* screen, not room state, so nothing
about it goes on the wire) backs a slider on the fog family's strip, GM-only in practice since the
whole strip already is. `renderFog` takes the opacity as a parameter instead of a hardcoded 0.5;
the GM's cover renders at whatever they've set, clamped to [0.15, 0.9] so it can never be tuned
down to invisible or up to fully opaque — both would defeat the point of a GM-visible cover at all.

Considered and dropped during implementation: a dashed outline around every revealed cell while
the fog family is the active tool, meant to make boundaries easy to trace while painting. Screenshot
verification showed it just re-traced the grid lines with no new information whenever most of the
scene was already revealed — which is now the common case, precisely because of change #2 above.
The opacity slider already gives a GM direct, adjustable control over how visible fog is; the
outline would have been a second, noisier way to chase the same goal. Not implemented.

## Related user stories

- [gm-hide-fog-cell](../user-stories/gm-hide-fog-cell.md)
- [gm-reveal-entire-scene-fog](../user-stories/gm-reveal-entire-scene-fog.md)
