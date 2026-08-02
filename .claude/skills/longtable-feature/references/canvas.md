# The Konva canvas

`web/src/lib/components/game-canvas.svelte`. One Konva `Stage`, one `<canvas>` element per layer,
world coordinates in pixels with the map's top-left at the origin.

## Layers

Added in this order, so this is also the `document.querySelectorAll('canvas')` index order:

| Index | Layer | Listening | Holds |
| --- | --- | --- | --- |
| 0 | map | yes | the map image, or a flat rect when there's no image |
| 1 | grid | no | grid lines, recomputed for the visible region on every pan/zoom/resize |
| 2 | fog | yes | the cover, with revealed cells punched out (`destination-out`) |
| 3 | drawings | no | committed strokes |
| 4 | tokens | yes | one `Group` per token, draggable only in `'none'` mode |
| 5 | pings | no | pulse rings |
| 6 | measurements | no | in-progress measurements from anyone in the room |
| 7 | preview | no | the current rubber-band shape, cursor ring, eraser halo |
| 8 | selection | no | the rotating ring around the selected token, if any |

Several Playwright specs read pixels from a layer *by index* (`DRAWING_LAYER = 3`,
`PING_LAYER = 5`, `MEASURE_LAYER = 6`, `SELECTION_LAYER = 8`). Appending a layer is safe;
inserting one renumbers everything above it. Either way, update the layer-order comments in
`web/e2e/*.spec.ts` — they're the only documentation of that coupling.

The selection ring has a layer to itself for a reason worth keeping: it's spun by a
`Konva.Animation`, which redraws its whole layer every frame for as long as it runs. On the token
layer that would be a 60fps rebuild of every token whenever anything was selected — the same
shape as the two lag bugs in `planning/backlog/done/`.

Konva warns above 5 layers ("Recommended maximum number of layers is 3-5"). Expected here and
harmless; the separation is what keeps a stroke from forcing a token re-render.

## Tools

`Tool` is `'none' | 'fog-reveal' | 'fog-hide' | DrawingKind | 'ping' | 'eraser' | 'measure' |
'template-circle' | 'template-cone' | 'template-line' | 'template-cube'`.
`'none'` is plain pan/token-drag; every other tool takes the stage's pointer exclusively, because
they all interpret a left-drag differently. The toolbar lives in
`web/src/routes/r/[slug]/+page.svelte` and toggles — clicking the active tool returns to `'none'`.

The two fog tools share one branch in `attachToolHandlers`: same sweep over the same cells,
differing only in which command the gesture ends with and the colour it previews in. The
whole-scene fog actions (`Reveal all`, `Reset fog`) are plain buttons rather than tools —
neither has a gesture to make, and making them modes would arm something that fires on the next
click anywhere on the map.

The four area templates share the *measuring tool's* branch, since they are the same gesture with
a different shape on the wire (see `$lib/aoe`, and its header comment on why nothing highlights
the squares a template covers). Their options — snap mode, and a Line's width — live in a row
that only appears while a template tool is active. Note that row changes the page height, which
moves the canvas: an e2e spec must re-read `canvasOrigin` after selecting a template tool, or
every drag it makes is silently offset.

**Anything a handler needs from a prop has to be read in `attachToolHandlers` itself, not inside
the handler closure.** The function runs inside the `$effect` that rebinds handlers, so only what
it reads *synchronously* is tracked; a prop read later, when a pointer event fires, is captured
once and never refreshed. That's why `snapMode` is copied to a local before the closures are
built — read in place, the snap control did nothing until the tool was reselected.

Selecting a token is *not* a tool — it's a plain click, bound in the same function under a
`.select` namespace and only while `activeTool` is `'none'`, since with a tool active a click
means erase, ping, or the first half of a drag. The handler walks up from `e.target` to a group
named `token` (`findAncestor('.token', true)`), so a click on the art, the placeholder circle or
its initials all resolve to the same token, and anything else — grid, map image, a drawing —
clears the selection. Konva suppresses `click` after a real drag, which is what keeps dragging a
token from also selecting it. Which token is selected is a `$bindable` prop, not `RoomClient`
state: nothing about it goes on the wire.

**A known consequence of rebuilding the token layer wholesale: a click can be lost.** Konva only
fires `click` when `mousedown` and `mouseup` land on the *same* node, and `renderTokens` destroys
and recreates every group whenever `room.tokens` changes — so a `token.moved` echo arriving
between the two halves of a click silently swallows it. The window is about a frame, and in
practice you click again; fixing it properly means diffing the token layer instead of rebuilding
it, which this file argues against everywhere else. It matters most in tests, where a click
immediately after a drag hits it roughly three runs in four — see `token-delete.spec.ts`, which
selects before dragging for exactly this reason.

`attachToolHandlers()` runs in an `$effect` on `activeTool`/`scene`/`you` and is the single place
pointer handlers are bound. It:

1. Removes every `.tool`-namespaced handler (`mousedown.tool touchstart.tool …`).
2. Resets all transient gesture state — and *retracts* anything in flight. A measurement or fog
   sweep abandoned by a tool switch has to be ended here, or it strands on other clients with no
   end event coming.
3. Sets `stage.draggable(!isActive)`, since panning and tools both start on a left-drag.
4. Binds the handlers for the active tool and returns early per tool.

Handlers use `stage.getRelativePointerPosition()`, not the raw event coordinates — that's what
accounts for the stage's own pan and zoom. `mouseleave.tool` matters for any held gesture: no
`mouseup` arrives if the button is released outside the canvas.

**Every handler that opens or closes a gesture checks `isPrimaryPointer(e)` first.** Konva reports
all mouse buttons through the same `mousedown`/`mouseup`, so without it a right-drag drives the
active tool exactly as a left one does. Both ends need it: guarding only `mousedown` still lets a
stray right-click *during* a left-button gesture fire `mouseup` and commit it early. `mouseleave`
is the deliberate exception — it must end a held gesture whatever the buttons are doing. Note the
helper tests "is a `MouseEvent` *and* not button 0" rather than `button !== 0`, because
`TouchEvent` has no `button` at all and the shorter spelling rejects every touch.

Adding a tool: extend the `Tool` union, add a branch in `attachToolHandlers`, add a toolbar button
with a distinct `aria-label` (the e2e helpers select by accessible name, and assert the active
styling `bg-primary` before dragging — the rebinding happens in an effect, so a click in the same
tick can land on the old tool). The rubber-band drawing branch at the end of `attachToolHandlers`
now names `line`/`rect`/`ellipse` explicitly instead of taking whatever is left over: as a
fall-through, a tool added above without its own branch silently became a drawing tool.

## Screen pixels vs world units

Anything that should read the same at every zoom level is authored in screen pixels and converted
with `screenToWorld(px)` — `px / stage.scaleX()`. Grid lines, stroke widths on overlays, the
eraser's reach, measurement dashes and label text all do this.

The consequence: **a zoom changes what those values mean, so it has to re-render them.**
`handleWheel` explicitly calls `renderGrid()`, `renderMeasurements()` and
`refreshCursorOverlay()` for exactly that reason. A new screen-sized overlay needs the same
treatment or it silently mis-sizes after a scroll.

Cell arithmetic: `cellAt`/`cellCentre` in `web/src/lib/measure.ts` floor a world point onto its
cell and find a cell's centre. Tokens are stored in cell units and multiplied by `gridSize` at
render time; drawings are stored in world units. Note `scene.gridOffsetX/Y` are **dead** — stored
and sent but never applied; grid alignment is being handled at asset upload time instead.

## Effects and re-render cost

`render()` is `async` and awaits the map image, so Svelte's dependency tracking — which only sees
reads before the first `await` — would miss `fogCells` and `you` entirely. The `track(...)` helper
forces those reads into the synchronous window. Any new async render path needs the same.

The effects are split by cost on purpose:

- `render()` tracks scene/fogCells/you — the expensive full rebuild.
- `renderDrawings()` has its own effect, because drawing and erasing are the most frequent things
  that happen and rebuilding the map, grid, fog and every token for one stroke was a real
  performance bug (`planning/backlog/done/erasing-causes-canvas-lag.md`).
- `renderTokens()` has its own effect for exactly the same reason, dragging a token having been
  the same bug a second time (`planning/backlog/done/token-drag-causes-canvas-lag.md`). It is
  also where `activeTool` is tracked, because token draggability is the only thing in the whole
  render path that reads it — pairing it with `render()` made every tool switch redraw the map.
- Pings, measurements and the eraser's halo each get their own.

Both of those effects call the same render function `render()` calls, so a scene change runs it
twice. That is deliberate: a scene change is rare, and the alternative — teaching `render()` to
skip the layers that own themselves — puts the ordering back in one place where the next
frequently-changing collection would have to be remembered.

Follow that pattern: a frequently-changing collection deserves its own effect and its own layer.
Rebuilding a whole layer wholesale is fine when it holds a handful of shapes; diffing is only
worth it for collections that accumulate.

## Optimistic rendering

Drawings appear the instant the stroke ends rather than after the round trip: the preview shape is
destroyed at `mouseup`, so waiting for the server would blink the line off and back on. The id is
minted client-side so the echo can be recognised. The eraser is optimistic for the same reason —
it highlights what a click will remove, so leaving the stroke there until the server agrees reads
as a missed click.

Refusals come back as an `error` naming the `drawingId`, and `RoomClient` either restores the
erased stroke *at the index it held* or removes the one it drew. Anything new that renders ahead
of the server needs the same two-way handling.

## Hit-testing

The eraser doesn't ask Konva what was clicked — the drawings layer is inert (`listening: false`)
and `web/src/lib/drawing-hit.ts` finds the nearest drawing geometrically, within a reach expressed
in screen pixels. That's why a thin stroke is no harder to hit than a thick one, and why the
eraser interpolates along the path between pointer events instead of testing only where they
landed (fast mouse moves arrive far apart and would sweep straight over strokes in between).

Permission is checked client-side too (`canErase`), so clicking someone else's work is a no-op
rather than an error toast — the server enforces it regardless.
