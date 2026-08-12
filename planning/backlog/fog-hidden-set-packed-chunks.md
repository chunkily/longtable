---
title: Store fog as the hidden set, packed 32 cells to an integer
created: 2026-08-12
status: done
tags: [fog, storage, protocol]
---

Fog was one row per **revealed** cell in `fog_cell`, which is the inverse of what the app
actually needs and roughly 32x more rows than it needs to be.

Two problems, one cause. Revealed-by-default meant a scene had to *materialise* a revealed cell
for every square in its bounds just to come up looking normal — added in
[fog-rect-painting-and-defaults](fog-rect-painting-and-defaults.md), capped at 40,000 cells, and
silently giving up on maps larger than that (which then came up fully covered, the exact bug the
default was introduced to fix). And a fully covered 200x200-cell map was 40,000 rows in SQLite
*and* 40,000 `{x,y}` objects in the payload every client receives on join, on scene switch, and
on reset.

- [ ] Store what's hidden, so the common state (revealed) costs no rows
- [ ] Pack cells into bitmask chunks rather than a row each
- [ ] Carry chunks over the wire too, not just in the table

## What shipped

`fog_cell` is gone, replaced by `fog_mask` — one row per `(scene_id, cell_y, chunk_x)` holding a
32-bit `mask` of **hidden** cells, bit *n* being the cell at `x = chunk_x*32 + n`. An absent row
(and a row that would reach mask 0, which is deleted rather than kept) means those 32 cells are
revealed. `internal/store/fog.go` and `web/src/lib/fog.ts` are the two halves of the format and
have to agree bit for bit.

The chunks go over the wire unexpanded: `state.sync` and `scene.activated` carry `fogChunks`, and
`fog.revealed`/`fog.hidden` carry `{sceneId, chunks}` where each chunk is the mask it now *has*
(not a delta). The two painting commands still take cells, because cells are what the rectangle
tool paints in and the server is the right place to group them.

Decisions worth not rediscovering:

- **The cap and the bounds check swapped buttons.** `fog.revealAll` is now a `DELETE` that only
  has to describe chunks which actually hold fog, so it needs no scene bounds and has no cap — it
  works on a scene with no width/height at all, which it used to refuse. `fog.reset` is the one
  that enumerates bounds now, so `maxFogChunks` and the "no map bounds" refusal live there. An old
  comment claiming the opposite is describing the previous scheme.
- **`fog.reset` stopped being an event.** Covering everything is `fog.hidden` over every chunk,
  and uncovering everything is `fog.revealed` with every chunk zeroed, so both whole-scene buttons
  reuse the painting events and the client has one merge rule rather than three cases.
- **Reset's delta includes chunks outside the scene's bounds, zeroed.** `HideAllCells` replaces
  the whole scene's fog, so fog painted left of or above the origin gets deleted — and without
  reporting those chunks zeroed, a client would keep drawing fog the server no longer has.
- **Shift and mask, never divide and modulo.** Go's `/` and `%` and JavaScript's truncate toward
  zero, so x=-1 would fold into chunk 0 alongside x=1 at a colliding bit. `x >> 5` floors. The
  grid is infinite and fog *can* be painted at negative coordinates; both sides have a test for it.
- **Painting broadcasts only chunks whose mask actually changed.** The rectangle tool sends every
  cell in its box including ones already in the target state, so a drag over already-correct
  ground now says nothing at all rather than echoing a no-op to every client.
- **Rendering inverted with the storage.** `renderFog` fills the hidden cells directly instead of
  covering the scene and punching revealed cells out with `destination-out`. A scene with no fog
  draws nothing. Every run goes into one `Konva.Shape` as a single compound path rather than a
  rect each — abutting translucent rects double-blend along shared edges and leave a hairline grid
  over the fog at any opacity below 1.

`fog_cell` is dropped rather than migrated (`DROP TABLE IF EXISTS` in the schema): it held the
complement of what `fog_mask` holds, so converting would mean inverting every row against scene
bounds that unbounded scenes don't have. **Painted fog in an existing database is lost on
upgrade** — accepted deliberately for a pre-1.0 self-hosted tool rather than worth a migration
path.

## Related user stories

- [gm-hide-fog-cell](../user-stories/gm-hide-fog-cell.md)
- [gm-reveal-entire-scene-fog](../user-stories/gm-reveal-entire-scene-fog.md)
- [gm-reset-scene-fog](../user-stories/gm-reset-scene-fog.md)
