# The Konva canvas

`web/src/lib/components/game-canvas.svelte`. One Konva `Stage`, one `<canvas>` element per layer,
world coordinates in pixels with the map's top-left at the origin.

## Layers

Added in this order, so this is also the `document.querySelectorAll('canvas')` index order:

| Index | Layer | Listening | Holds |
| --- | --- | --- | --- |
| 0 | map | yes | the map image, or a flat rect when there's no image |
| 1 | grid | no | grid lines, recomputed for the visible region on every pan/zoom/resize |
| 2 | fog | yes | one compound-path shape covering the hidden cells (see below) |
| 3 | drawings | no | committed strokes |
| 4 | tokens | yes | one `Group` per token, draggable in `'none'` mode and only if `room.canMoveToken` |
| 5 | pings | no | pulse rings |
| 6 | measurements | no | in-progress measurements from anyone in the room |
| 7 | preview | no | the current rubber-band shape, cursor ring, eraser halo |
| 8 | selection | no | the rotating ring around the selected token, if any |
| 9 | hover | no | the card of trackers and conditions for the token under the pointer |

Several Playwright specs read pixels from a layer *by index* (`DRAWING_LAYER = 3`,
`PING_LAYER = 5`, `MEASURE_LAYER = 6`, `SELECTION_LAYER = 8`, `HOVER_LAYER = 9`). Appending a
layer is safe; inserting one renumbers everything above it. Either way, update the layer-order
comments in `web/e2e/*.spec.ts` — they're the only documentation of that coupling.

The selection ring has a layer to itself for a reason worth keeping: it's spun by a
`Konva.Animation`, which redraws its whole layer every frame for as long as it runs. On the token
layer that would be a 60fps rebuild of every token whenever anything was selected — the same
shape as the two lag bugs below.

The hover card earns one for the neighbouring reason: `renderTokens` destroys and rebuilds the
token layer wholesale on *any* change to `room.tokens`, so a card living there would blink out
every time anyone moved anything. Which token is hovered is `$state` (`hoveredTokenId`), unlike
the selection ring's plain-`let` bookkeeping, because the card is rendered from an effect rather
than kept alive across renders — here the id genuinely is the reactive truth. A token with no
trackers and no conditions gets **no card at all**: every token popping an empty box as the
pointer crossed it would make the map unusable during a fight, which is when this is for.

Konva warns above 5 layers ("Recommended maximum number of layers is 3-5"). Expected here and
harmless; the separation is what keeps a stroke from forcing a token re-render.

## Tools

`Tool` is `'none' | 'fog-reveal' | 'fog-hide' | DrawingKind | 'ping' | 'eraser' | 'measure' |
'template-circle' | 'template-cone' | 'template-line' | 'template-cube'`, and lives in
`web/src/lib/tool-family.ts` rather than in the canvas — the canvas only cares which tool is
active, never how the toolbar arranges them. `'none'` is plain pan/token-drag; every other tool
takes the stage's pointer exclusively, because they all interpret a left-drag differently.

The toolbar (`$lib/components/map-toolbar.svelte`) shows **five families** — hand, draw, measure,
fog, ping — and the active family's variants and settings go on a contextual strip
(`$lib/components/tool-strip.svelte`) that the room page places separately: floating under the
tool row on a desktop, docked into the bottom sheet on a phone. `'none'` *is* the hand, so the
family is **derived** from the active tool (`familyOf`) rather than stored beside it — two pieces
of state could disagree, and there is no question the pair answers that the tool can't answer
alone.

Neither family nor variant buttons toggle any more: the Hand is how you stop, so a second click on
the active family would be a surprising second way to do the same thing. `toolForFamily` remembers
what each family was last left on, so returning to Draw puts back the shape you were using.

The two fog tools share one branch in `attachToolHandlers`: same rectangle rubber-banded over the
same cells, differing only in which command the gesture ends with and the colour it previews in.
A drag with no movement collapses to the single cell under the pointer, so a touch-up still
works. Only
templates snap or quantise — the distance line already reports whole squares from the cells its
ends fall in, and is a ruler rather than a spell. The
whole-scene fog actions (`Reveal all`, `Hide all`) are plain buttons rather than tools —
neither has a gesture to make, and making them modes would arm something that fires on the next
click anywhere on the map.

The four area templates share the *measuring tool's* branch, since they are the same gesture with
a different shape on the wire (see `$lib/aoe`, and its header comment on why nothing highlights
the squares a template covers). Their options — snap mode, and a Line's width — sit on the measure
family's strip and appear only once a template is chosen, since neither applies to the plain
ruler. The ruler's own button is labelled **Distance**, not Measure: the family button is already
called Measure, and two controls with the same accessible name in one view is ambiguous to a
screen reader and a test runner alike.

The two ends of a template drag are treated differently, and only for templates. The **origin**
obeys the snap mode; the **far end** is not snapped but quantised — its direction is taken as
dragged and its length rounded to the nearest 5 ft, because every printed area is a multiple of
5. Snapping the far end as well would only coarsen the direction, and the quantise would move it
off the grid regardless. Note snapping the origin alone never fixed this: a one-square diagonal
between two corners is 5·√2 ≈ 7.07 ft.

Since the full-bleed layout the canvas fills the window, so the toolbar and its strip **float over
the map's top-left corner** — roughly the first 380×110px belongs to the toolbar, not the map. A
pointer gesture starting inside that lands on a button, which reads as "drawing silently stopped
working" rather than as a mis-aimed test. `e2e/room.ts` exports `mapGestureOrigin` (the canvas
corner plus `TOOLBAR_CLEARANCE_Y`) for exactly this; a spec that *also* probes pixels has to add
the same clearance back on, because the canvas's own buffer still starts at its true corner. The
strip no longer changes the page height — it floats — so a spec doesn't have to re-read the origin
after picking a template.

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

**A token the room's lock says you may not move swallows `mousedown` rather than merely being
undraggable**, and that is not tidiness. Konva starts the *stage* drag from whatever pointerdown
bubbles up to it, so a group with `draggable: false` hands the gesture straight to the map: the
first version of the movement lock panned the whole scene every time a Player grabbed somebody
else's token, which reads as the app misbehaving rather than as "this one isn't yours". Setting
`e.cancelBubble = true` in a `mousedown.lock`/`touchstart.lock` handler stops it. `click` is a
separate event and still bubbles, so a locked token can be selected and inspected as before —
which is also why selection had to be checked separately when this went in.

`attachToolHandlers()` runs in an `$effect` on `activeTool`/`scene`/`you` and is the single place
pointer handlers are bound. It:

1. Removes every `.tool`-namespaced handler (`mousedown.tool touchstart.tool …`).
2. Calls `retractInFlightGesture()` — resets transient gesture state and *retracts* anything in
   flight. A measurement or fog rectangle abandoned by a tool switch has to be ended there, or it
   strands on other clients with no end event coming. It's a named function rather than an inline
   block because a pinch needs identical cleanup (below), and two copies is how a retraction gets
   added to one and missed in the other.
3. Sets `stage.draggable(!isActive)`, since panning and tools both start on a left-drag.
4. Binds the handlers for the active tool and returns early per tool.

**The pinch handlers are `.pinch`-namespaced and bound once in `onMount`, not here** — step 1
would tear them off on every tool change, and a pinch has to work whatever tool is selected. When
a second finger lands, `handlePinchMove` calls `stage.stopDrag()` (or the map pans and scales at
once and slides out from under the fingers) and then `retractInFlightGesture()`: a tool owns the
pointer and a pinch is two of them, so the second touch would otherwise read as more of the same
stroke. The gesture is abandoned rather than committed — a half-drawn line that appears because
someone reached to zoom isn't something they asked for.

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

Adding a tool: extend the `Tool` union in `$lib/tool-family.ts`, put it in a family there (and in
`DRAW_TOOLS`/`MEASURE_TOOLS`/`FOG_TOOLS` if it belongs to one), add a branch in
`attachToolHandlers`, and add a button to that family's strip in `tool-strip.svelte` with a
**distinct** `aria-label` — distinct from the family names too, which is why the ruler is
`Distance`. The e2e helpers select by accessible name and assert the active styling `bg-primary`
before dragging, since the rebinding happens in an effect and a click in the same tick can land on
the old tool; teach `TOOL_FAMILY` in `e2e/room.ts` which family the new tool sits under or
`selectTool` won't find it. The rubber-band drawing branch at the end of `attachToolHandlers`
now names `line`/`rect`/`ellipse` explicitly instead of taking whatever is left over: as a
fall-through, a tool added above without its own branch silently became a drawing tool.

## Screen pixels vs world units

Anything that should read the same at every zoom level is authored in screen pixels and converted
with `screenToWorld(px)` — `px / stage.scaleX()`. Grid lines, stroke widths on overlays, the
eraser's reach, measurement dashes and label text all do this.

The consequence: **a zoom changes what those values mean, so it has to re-render them.** That
whole set lives in **`applyViewChange()`** — `renderGrid`, `renderMeasurements`,
`renderSelection`, `renderHoverCard`, `refreshCursorOverlay` — and every path that alters the
stage's scale or position calls it: `handleWheel`, the pinch handler, `resetView`, **and the
`ResizeObserver`**. The observer is on that list since the full-bleed layout: the stage now fills
the window, so it resizes for reasons that aren't a window drag — the mobile sheet opening, the
contextual strip appearing — and each of those moves the viewport over the world exactly as a pan
does. A new screen-sized overlay gets a line in there, or it silently mis-sizes after the first
zoom.

**`resetView` calls it through `untrack()`, and that matters.** `resetView` runs inside
`render()`'s effect (on a scene change) *before* its first await, so the reads inside
`applyViewChange` — `renderSelection` reaches `selectedTokenId` — land in the window Svelte is
collecting dependencies in. Untracked, the map effect gains a dependency on the selection:
clicking a token rebuilds the map, which re-enters `resetView`, and selection stops working
across most of the suite. Anything else called from inside `render()`'s synchronous window that
reads state belonging to another effect needs the same treatment.

Cell arithmetic: `cellAt`/`cellCentre` in `web/src/lib/measure.ts` floor a world point onto its
cell and find a cell's centre. Tokens are stored in cell units and multiplied by `gridSize` at
render time; drawings are stored in world units. Note `scene.gridOffsetX/Y` are **dead** — stored
and sent but never applied; grid alignment is being handled at asset upload time instead.

## Fog

The fog layer draws the **hidden** cells directly rather than covering the scene and punching the
revealed ones back out — matching the storage, which is the hidden set (see `ws-protocol.md`). A
scene with no fog therefore draws nothing at all.

Every hidden run goes into **one** `Konva.Shape` as one compound path, filled once, rather than a
rect per run. Abutting translucent rectangles blend twice along the edge they share and leave a
hairline grid over the fog at any opacity below 1; a single path has no interior edges to blend.
`fogRuns` in `web/src/lib/fog.ts` unpacks chunks into those runs and deliberately stops them at
chunk boundaries — abutting rects in one path union cleanly, so merging across would buy nothing.

A Player's fog is opaque. A GM's uses their own `fogOpacity`, a per-browser preference in
`$lib/fog-opacity` (not room state, so it never goes on the wire) surfaced as a slider on the fog
family's strip.

## Effects and re-render cost

`render()` is `async` and awaits the map image, so Svelte's dependency tracking — which only sees
reads before the first `await` — would miss `scene` and `you` entirely. The `track(...)` helper
forces those reads into the synchronous window. Any new async render path needs the same.

The effects are split by cost on purpose:

- `render()` tracks scene/you — the expensive full rebuild.
- `renderFog()` has its own effect, tracking `fogChunks` and the GM's `fogOpacity`. The opacity
  fires on every tick of a slider drag, and rebuilding the map, grid and every token for that
  would be the same performance bug the two below record.
- `renderDrawings()` has its own effect, because drawing and erasing are the most frequent things
  that happen and rebuilding the map, grid, fog and every token for one stroke was a real
  performance bug (`planning/backlog/erasing-causes-canvas-lag.md`).
- `renderTokens()` has its own effect for exactly the same reason, dragging a token having been
  the same bug a second time (`planning/backlog/token-drag-causes-canvas-lag.md`). It is
  also where `activeTool` is tracked, because token draggability is the only thing in the whole
  render path that reads it — pairing it with `render()` made every tool switch redraw the map.
- Pings, measurements and the eraser's halo each get their own.
- The colour scheme gets one, and it is the only place `mode.current` is read. Everything else
  reaches the scheme through `stageScheme`, a plain `let` — see below.

Both of those effects call the same render function `render()` calls, so a scene change runs it
twice. That is deliberate: a scene change is rare, and the alternative — teaching `render()` to
skip the layers that own themselves — puts the ordering back in one place where the next
frequently-changing collection would have to be remembered.

Follow that pattern: a frequently-changing collection deserves its own effect and its own layer.
Rebuilding a whole layer wholesale is fine when it holds a handful of shapes; diffing is only
worth it for collections that accumulate.

**Animating across a wholesale rebuild.** Tokens slide to a new square rather than jumping, which
looks like it needs node identity preserved across renders — and it doesn't. `renderTokens`
remembers where it last drew each token (`renderedPositions`, world units), builds the new group
at *that* position, and tweens it to where the token now belongs. The rebuild is untouched. Three
things that fall out of it, all already handled:

- The dragger records the snapped position in `dragend`, before the echo. Otherwise the broadcast
  comes back, finds the token still remembered at the square it left, and slides it the whole way
  a second time.
- A re-render mid-slide reads each group's *current* position first (`midSlide`), so an unrelated
  token moving doesn't snap a travelling one back to where its slide started.
- A scene change clears the memory, or every token slides in from wherever some unrelated token
  stood on the previous map.

These tweens are transient, unlike the selection ring's `Konva.Animation`, which is why they can
live on the token layer rather than earning one of their own. `prefers-reduced-motion` turns them
off.

## Light and dark on the stage

Two things Konva paints follow the app's colour scheme, and only two: `MAP_PLACEHOLDER`, the slab
shown where a scene has no map image, and `GRID_LINE`. Both sit against the container's
`bg-muted`, which flips with the theme, so a black 13%-opacity grid disappears entirely on a dark
background. Everything else painted here — strokes, pings, measurements, the eraser's halo, the
selection ring — is *map content* and stays put in both schemes; the dark-map drawing palette is a
separate problem with its own backlog item.

They're explicit `{ light, dark }` pairs rather than reads of the CSS custom properties, because a
canvas takes colour strings and has never heard of `var()`.

**The scheme is read reactively in exactly one effect**, which assigns a plain `let stageScheme`
and calls `render()` itself. The render functions read that variable, never `mode.current`. This
is the same hazard `resetView` documents above: the render functions run inside half a dozen
different effects, and one reactive read on the way past would give every one of them a dependency
on the theme. `stageScheme` is seeded from the current scheme rather than defaulting to light —
defaulting meant a dark browser rendered light first and immediately re-rendered, putting two
loads of the same map image in flight at once.

A full `render()` for a colour change is more than strictly needed and costs nothing: themes flip
a handful of times an evening, and the map image is already in `imageCache`. Doing less would mean
maintaining a second list of which render functions read the scheme.

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
