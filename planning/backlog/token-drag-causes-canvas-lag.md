---
title: Dragging a token causes lag when a map is loaded
created: 2026-07-31
status: done
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
- **`renderMap` stacked duplicate map images.** Found while measuring this, but a separate bug
  that predates both lag fixes, so it got its own note: `duplicate-map-image.md`. The short
  version is that `render()` was called twice at startup, and `renderMap` clears before its await
  and adds after it, so the map layer kept two copies of the bitmap. Fixed in the commit after
  this one.

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
