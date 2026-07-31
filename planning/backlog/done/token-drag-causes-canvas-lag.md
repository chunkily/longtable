---
title: Dragging a token causes lag when a map is loaded
created: 2026-07-31
tags: [tokens, performance]
---

Dragging a token around a scene with a map image loaded lags visibly. Reported on Linux, but not
specific to it — the same thing happens on Windows.

## Cause

The same bug as `erasing-causes-canvas-lag.md`, in the same effect, one collection over.
`room.tokens` was tracked by the effect that runs the whole `render()`, so moving one token
re-created the map image, every grid line, every fog cell and every drawing.

Measured against a scene with a 3500x2500 map, counting per-layer `destroyChildren`
(`rebuild`) and `batchDraw`/`draw` (`redraw`) calls for a single `token.moved`:

```
rebuild  map:1 grid:1 fog:1 drawings:1 tokens:1
redraw   map:3 grid:22 fog:3 drawings:2 tokens:5
new Konva.Image over the map bitmap: 1
```

A fresh `Konva.Image` over the bitmap per token move is the part that is felt, which is why this
shows up with a map loaded and not on the blank scenes the tests use — exactly as it did for
erasing.

The drag itself is not what lags. `token.move` is only sent at `dragend`, so the rebuild lands on
the *drop*: drag, hitch, drag, hitch.

## Fix

Tokens got their own effect, alongside the one drawings already had:

```
rebuild  tokens:1
redraw   tokens:5
new Konva.Image over the map bitmap: 0
```

35 layer redraws across five layers down to 5 on one, and the map bitmap no longer among them.

`activeTool` moved to the token effect at the same time. Token draggability is the only thing in
the whole render path that reads it, so tracking it beside `render()` meant every tool switch
redrew the map too.

`token.moved` also stopped reassigning `tokens` when the event changes nothing. A token dropped
back on the cell it started from still broadcasts, and the hub only ever broadcasts coordinates it
accepted — a refusal comes back as an `error` instead — so an event matching what the client
already holds is always redundant. It was rebuilding every token group, which briefly destroys the
group under the pointer if a second drag has already begun.

## Not done

Two things found while measuring, both left alone:

- `renderTokens` still destroys and rebuilds every token group for any change, so moving one
  token of N rebuilds the other N-1. Now confined to one cheap layer, same as `renderDrawings`.
  The fix, if a scene ever holds enough tokens to matter, is a map of id to group.
- **`renderMap` stacks duplicate map images.** The map layer holds two identical `Konva.Image`s
  over the same bitmap from the first load onward, doubling the cost of every map redraw. This
  predates both lag fixes — the pre-fix build does it too — and it is now off the token path, but
  it still costs on every pan, zoom and fog change.

  It is a deterministic double-invocation, not a flaky race. `render()` has two call sites that
  both fire at startup: `onMount` calls it directly, and the first run of the `$effect` calls it
  again, in the same flush. `renderMap` clears synchronously and adds after an `await`, so both
  clears land before either add and neither image removes the other. `imageCache` is only written
  in `img.onload`, so both calls miss it and each starts its own fetch — the giveaway is **two
  HTTP requests for the same asset URL on one page load**, which is how this was confirmed.

  Two things that do *not* reproduce it, and why they mislead: switching scenes to a fresh
  uncached map draws exactly one image, because only the effect fires; and two `fog.reveal`s in a
  row also draw one each, because separate socket messages land in separate macrotasks and the
  renders never overlap. Only the startup pair is concurrent.

## Verifying this kind of thing

There is no harness for it, so for the record: build a binary from the current source and one from
the parent commit, run them on two spare ports with `-addr/-db/-assets` pointed at throwaway
paths, seed a room with a large map through the REST API and `scene.create`/`token.create` over
the socket, then wrap `Konva.Layer.prototype.batchDraw` and `Konva.Container.prototype.destroyChildren`
to count per layer and drive moves from a second socket.

Counting `clearRect` on the layer canvases — how `erasing-causes-canvas-lag.md` was measured — only
works in a browser that is actually compositing. `batchDraw` defers the real drawing to
`requestAnimationFrame`, which does not fire in a hidden or non-compositing window, so the scene
graph updates and the pixels never move and every count comes back zero. Counting the scheduling
calls instead is what makes the measurement work headless.
