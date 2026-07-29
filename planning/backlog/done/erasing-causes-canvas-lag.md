---
title: Erasing items on the canvas causes lag
created: 2026-07-30
tags: [drawing, performance]
---

When user erases an item on the map, a short lag occurs until the object is
deleted before the canvas is interactable again.

## Cause

Two compounding problems, found by counting `clearRect` calls per layer canvas during an erase.
Konva clears a layer's canvas each time it redraws it, so the count says exactly what was rebuilt:

```
erase one stroke → map:2 grid:2 fog:2 drawings:2 tokens:2 preview:2
```

- `room.drawings` was tracked by the effect that runs the whole `render()`, so changing one stroke
  re-created the map image, every grid line, every fog cell and every token group. On a scene with
  a real map image that means a full `drawImage` of the bitmap per erase, which is why this was
  felt on real maps and not on the blank test scenes.
- Everything ran twice: once for the optimistic removal, then again when `drawing.deleted` arrived
  and reassigned the list even though the stroke was already gone.

## Fix

Drawings got their own effect, and the confirmation is ignored when it changes nothing:

```
erase one stroke → drawings:1 preview:1
```

Twelve layer redraws down to two, with the map bitmap no longer among them. The same applies to
finishing a stroke, which previously rebuilt the whole canvas twice as well.

## Not done

`renderDrawings` still destroys and rebuilds every shape on the layer for any change, so a sweep
across N strokes rebuilds the survivors N times. That's now confined to one cheap layer, so it was
left alone — if a scene ever holds enough strokes for it to matter, the fix is to keep a map of id
to shape and add or remove only what changed.
