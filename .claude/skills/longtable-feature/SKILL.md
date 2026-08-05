---
name: longtable-feature
description: End-to-end recipe for building or changing anything that happens in a Longtable room — map tools, tokens, fog, scenes, chat, drawings, measurements, initiative. Covers the WebSocket command/event protocol in internal/ws, RoomClient runes state, the Konva canvas, and what to test at each layer. Use this whenever a task touches internal/ws/, web/src/lib/room.svelte.ts, or web/src/lib/components/game-canvas.svelte, or whenever a backlog item asks for a new map tool, a new thing to sync between players, or a change to how the canvas draws — even if the request sounds small ("stop right-click from drawing", "show HP on tokens"). The layering here is easy to get wrong from first principles, and there are several traps that only show up with two browsers open.
---

# Building a Longtable room feature

Everything that happens on a map crosses four layers. Work them in order — the wire format is the
decision everything else follows from, and it's the expensive one to change later.

1. **`internal/ws/hub.go`** — accept a command, validate it, apply it, broadcast an event.
2. **`internal/store/`** — only if the feature persists. Ephemeral features skip this entirely.
3. **`web/src/lib/room.svelte.ts`** — send the command, fold the event into runes state.
4. **`web/src/lib/components/game-canvas.svelte`** — render the state, bind the pointer handlers.

Then tests at three levels (Go hub, vitest `RoomClient`, Playwright with two browsers). See
[references/testing.md](references/testing.md) for what belongs at each level and the helpers that
already exist.

## First question: does this persist?

It decides most of the design, so settle it before writing anything.

**Persisted** (tokens, drawings, fog, scenes, chat) — needs a store table, has to appear in
`state.sync` so a fresh client sees it, and survives reload. Follow `handleDrawCreate` and
`internal/store/drawing.go`.

**Ephemeral** (pings, measurements) — never touches the store, never appears in `state.sync`,
and exists only while something is happening. The hub is a pure relay. Follow `handlePing` for a
one-shot and `handleMeasureUpdate` for a continuous gesture.

A continuous ephemeral gesture — anything dragged out and shown live to the room — has three
traps, all of them already solved in the measuring tool (`measure.update`/`measure.end`,
`web/src/lib/measure.ts`, and the `updateMeasure`/`endMeasure` pair in `RoomClient`):

- **Key it by participant, not by id.** One per person, every update replacing their last,
  ending with an explicit end command. A list that appends per pointer-move grows without bound.
- **Clean up on disconnect.** A client that drops mid-gesture never sends its end command, and
  whatever it was drawing hangs on everyone else's map. `ServeHTTP` broadcasts the end itself
  when the connection dies — note the deliberate defer ordering there.
- **Ignore your own echo.** The local copy is ahead of the wire, so folding your own broadcast
  back in drags the shape back to where your pointer used to be. Throttle sends with a trailing
  edge so the final position still lands.

## The protocol

Commands are lowerCamel, dot-namespaced by subject (`token.move`, `draw.create`,
`measure.update`). Events are the past tense of what happened (`token.moved`, `drawing.created`,
`measure.updated`). Payloads are JSON objects with camelCase keys, decoded into a
`fooRequest` struct next to its handler.

Register the command in `handleMessage`, then write the handler in the same style as its
neighbours. [references/ws-protocol.md](references/ws-protocol.md) has the full command/event
table, the permission and scoping helpers (`requireGM`, `requireSceneInRoom`,
`requireAssetInRoom`), how per-recipient filtering works for hidden tokens, and the error
conventions — read it before adding a command.

Two rules that are not negotiable, because they're what keeps one room out of another:

- **Identity comes from the connection, never the payload.** `c.participant.ID` /
  `c.participant.DisplayName`. A client can't claim to be someone else. This is why undoing an
  erase re-creates the drawing under whoever pressed undo.
- **Scope every id through the room.** `sceneInRoom`, `TokenRoomID`, `requireAssetInRoom`, and
  for a drawing, check its *own* scene rather than a scene id from the payload. A missing object
  and someone else's object answer identically, so a client can't probe for what exists
  elsewhere. Assets are the sharpest example: rows are global and content-addressed on purpose
  (dedup across rooms), so *existing* is never enough — the check has to be membership in the
  room's library (`room_asset`), or every room's asset IDs are usable by every other room.

## Client state

`RoomClient` is the only thing that touches the socket. Add a `$state` field for the new data, a
method that sends the command, and a `case` in `handleEnvelope`. Components read fields; they
never parse envelopes.

- `send()` returns whether the socket was actually open. Anything rendered optimistically must
  check it — a stroke drawn while the connection is down would otherwise sit on the map forever
  waiting for a round trip that never comes.
- Optimistic rendering is the norm for anything with a preview shape (see `sendCreate`/`sendErase`).
  Mint the id client-side so the echo can be matched to what's already on screen, and keep
  enough state to undo the optimism when the server refuses — an `error` event carrying a
  `drawingId` is how a refusal names what to take back.
- `resetAfterSync()` runs on `state.sync` and `scene.activated`. Anything scene-scoped or
  in-flight belongs there: after a full picture arrives, undo history and pending gestures refer
  to a scene that may no longer be on screen.
- Filter incoming scene-scoped events against `this.scene?.id` — events for other scenes still
  arrive.

## Canvas

Read [references/canvas.md](references/canvas.md) before editing `game-canvas.svelte`. The
essentials:

- **Layer order is an implicit contract.** Several Playwright specs read pixels from a layer by
  index. Adding a layer in the middle renumbers them — append rather than insert where you can,
  and update the layer-order comments in the specs either way.
- **One tool owns the pointer at a time.** `attachToolHandlers` tears down every `.tool`-namespaced
  handler and rebinds for the active tool, and disables stage dragging while a tool is live. Any
  in-flight gesture has to be retracted there, or switching tools mid-drag strands it.
- **Screen pixels vs world units.** Anything that should look the same at every zoom (stroke
  widths, eraser reach, label text) is authored in screen pixels and converted with
  `screenToWorld()` at render time — which means a zoom has to re-render it, not just redraw.
- **`$effect` and `await`.** `render()` awaits an image load partway through, so Svelte only sees
  dependencies read before the first await. The `track()` helper exists to force those reads into
  the synchronous window. Effects are also split deliberately by cost — drawings re-render on
  their own rather than rebuilding the map, grid, fog and tokens for one stroke.

## Working order

Land it in the order the layers depend on each other, verifying as you go:

1. Hub handler + Go test. `go test ./internal/ws/` — fast, and proves the wire format.
2. `RoomClient` method + envelope case + vitest cases for the state transitions.
3. Canvas rendering and pointer handlers.
4. `npm run check`, `npm run format`, `npm run lint`.
5. A Playwright spec, if the behaviour needs a real browser — anything involving two clients,
   canvas pixels, or a disconnect does.
6. Update the docs the change invalidated (below), in this same commit.
7. Move the backlog item and write its "What shipped" note (see the `longtable-backlog` skill).

## Leave the map matching the territory

This skill's references are only worth reading because they're true, and a feature is the thing
most likely to make them false. The repo is early and moves fast, so treat the doc edit as part
of the change rather than cleanup for later — a reference corrected next week has already sent
someone down the wrong path, which is exactly what happened with the eraser's `hitStrokeWidth`
note in `planning/backlog/eraser-tool.md`.

What a feature typically invalidates:

- **A new command or event, or a change to who may send one** → the command table in
  [references/ws-protocol.md](references/ws-protocol.md). It's the first thing anyone reads to
  learn what the protocol can do, so a missing row means a future session reinvents your command.
- **A new Konva layer or tool** → the layer table in [references/canvas.md](references/canvas.md),
  *and* the layer-order comments in `web/e2e/*.spec.ts`. Those specs index layers by number; a
  layer inserted anywhere but the end silently renumbers them.
- **A new test helper, or a change to how a suite runs** → [references/testing.md](references/testing.md),
  and `README.md` if the command a human types changed.
- **A shipped feature or a closed gap** → "Where things stand" in `CLAUDE.md`.
- **A `why` you had to work out the hard way** → a comment next to the code, in the house style.
  If it took you twenty minutes to understand, it will take the next session twenty minutes too.

Anything you find in these files that the code no longer does, fix as you pass — including in
`planning/done/` notes, where the correction goes *alongside* the original rather than replacing
it.

Pure geometry or rules — distance math, hit-testing — goes in its own module under
`web/src/lib/` with unit tests (`measure.ts`, `drawing-hit.ts`). It's the part that has to be
right and the part nobody can eyeball from a screenshot.
