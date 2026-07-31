---
title: The map layer holds two copies of the map image
created: 2026-07-31
tags: [canvas, performance]
---

Found while fixing `token-drag-causes-canvas-lag.md`. From the first page load onward the map
layer holds two identical `Konva.Image` nodes over the same bitmap, so every map redraw draws the
map twice. It predates both canvas lag fixes — a build from 8406fc5 does it too.

## Cause

A deterministic double-invocation, not a race on the network. `render()` had two call sites that
both fired at startup: `onMount` called it directly, and the first run of the `$effect` tracking
scene/fogCells/you called it again, in the same flush.

`renderMap` clears synchronously but adds after an await:

```js
mapLayer.destroyChildren();          // sync
const img = await loadImage(...);    // suspends
mapLayer.add(new Konva.Image({...})) // after
```

So the interleaving is destroy → destroy → add → add, and neither image removes the other.
`loadImage` only writes `imageCache` inside `img.onload`, so the second call misses a cache the
first has not populated yet and starts its own fetch. The giveaway is **two HTTP requests for the
same `/api/assets/<id>` on one page load**, which is how this was confirmed.

Two things that look like they should reproduce it and do not, which is what pins it on the
startup pair:

- Activating a different scene with a fresh, uncached map draws exactly one image — the
  destroy/add pair straddles a real suspension on the image load and still only one image,
  because only the effect fires. A slow await alone is not sufficient.
- Two `fog.reveal`s in a row also draw one image each: separate socket messages land in separate
  macrotasks, so those renders never overlap.

## Fix

Dropped the `render()` call in `onMount`. The effect already runs on mount, and effects run in
creation order — the `onMount` is declared above the effect, so it still runs first and the stage
exists by the time the effect renders. The pre-fix behaviour is itself the proof of that ordering:
two images could only appear if *both* calls got past `render()`'s `if (!stage) return`.

Guarding the await with a generation counter was the other option and was not taken. It would have
left the redundant second render in place, only to throw its work away.

Confirmed after the fix: one `/api/assets/<id>` request per load, and one child on the map layer.
