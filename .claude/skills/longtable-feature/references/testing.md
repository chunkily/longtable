# Testing a room feature

Three levels, each with helpers already in place. Match the existing files rather than inventing a
new harness.

## Go: the hub (`internal/ws/*_test.go`)

`newTestServer(t)` wires a real `Store` on a temp SQLite file to a `Hub` behind `httptest`, so
these exercise the actual protocol over a real socket rather than calling handlers directly.

```go
ts := newTestServer(t)
room, gm, _ := ts.store.CreateRoom("Room", "GM", "password")
player, _ := ts.store.JoinRoom(room.ID, "Bob")
scene, _ := ts.store.CreateScene(room.ID, "Scene", nil, 70, 10, 10)

client := ts.connect(t, room.Slug, player.SessionToken)
client.readEnvelope(t) // state.sync always arrives first
client.send(t, "measure.update", map[string]any{ /* … */ })
```

- `readEnvelope(t)` reads one envelope with a 2s timeout and fails the test on error.
- `expectNoMessage(t, d)` proves a recipient was *deliberately* skipped — the assertion for hidden
  tokens, where "nothing arrived" is the whole point.
- The sender gets its own broadcast echo, so read it off the sender before asserting on another
  client's copy.
- Group per-feature setup in a small struct with a constructor (`newDrawTestRoom`,
  `newMeasureTestRoom`) when more than a couple of tests need the same room.

Cover: the happy path and what it broadcast; a non-GM attempting a GM-only command; an id from
another room (must fail identically to one that doesn't exist); a malformed payload; and for
persisted features, that the store actually holds it afterwards — or for ephemeral ones, that it
*doesn't* (`ListDrawingsForScene` returning 0 is the assertion that a ping or measurement left
nothing behind).

Note `go test -race` needs cgo and can't run on Windows here; CI runs it on Linux.

## Vitest: `RoomClient` (`web/src/lib/room.svelte.test.ts`)

`FakeWebSocket` is a hand-rolled stand-in driven entirely from the test — `socket.emit(envelope)`
pushes a server event in, `socket.sent` is what went out. `connectedClient()` stubs the global and
returns both. Set `socket.readyState = 0` to simulate a closed connection, which is how the
"nothing was sent, so don't record it" cases are tested.

This is the level for state transitions: an event replacing rather than appending; scene-scoped
events for another scene being ignored; own-echo handling; a full `state.sync` clearing in-flight
state; throttles and timers (`vi.useFakeTimers()` inside a `try`/`finally` that restores real
ones).

Pure geometry and rules get their own module and their own spec — `measure.test.ts`,
`drawing-hit.test.ts`. Prefer putting logic there over inside the component, precisely so it can be
tested without a Konva stage.

## Playwright: two browsers (`web/e2e/*.spec.ts`)

For anything the DOM can't show: Konva rendering, multi-client sync, server-enforced permissions,
disconnects. `npx playwright test` builds the Go binary and starts both servers itself (see
`playwright.config.ts` and `e2e/run-backend.mjs`).

Reading the canvas is the standard trick — count opaque pixels on a layer, either in a small box
around a point (`inkAt`) or across the whole layer when the shape moves (`pingInk`, `measureInk`).
Remember `devicePixelRatio` when indexing into `getImageData`.

Two helpers worth copying verbatim rather than rewriting:

- `selectTool(page, name)` — clicks only if not already active, then waits for `bg-primary`.
  Tool handlers are rebound in a Svelte effect, so a drag sent in the same tick as the click can
  land on the previous tool.
- `canvasOrigin(page)` — canvas-relative pixels double as world coordinates, because a fresh scene
  starts at the identity transform. Don't pan or zoom in a spec that relies on that.

`selectTool` does **not** work for the fog tool, which relabels itself (`Reveal fog` →
`Painting fog…`) rather than only restyling: its locator stops matching the moment the tool goes
active, so the wait can never pass. `drawing-right-click.spec.ts` has a `selectFogTool` that
clicks the old label and waits on the new one, which is what proves the switch happened.

Two things that quietly make a pixel assertion measure nothing:

- **Counting non-transparent pixels is blind to a GM's fog.** A GM's cover is drawn at
  `opacity: 0.35` and revealed cells are punched out at `0.35` too, so revealing takes a pixel
  from roughly alpha 89 to roughly 58 — lower, never zero, and the count comes back identical.
  Sum the alpha channel instead (`layerAlpha`). Players get `opacity: 1` and a full punch-out, so
  the same probe works against a player's view; it's the GM rendering that defeats it.
- **The freehand tool always has ink on the preview layer**, because it paints a cursor ring
  sized to the stroke width that tracks the pointer whether or not a stroke is in progress.

Both of those produced passing tests that asserted nothing. Pair a "this must not happen"
assertion with a positive control on the *same* probe — the gesture that *should* work, checked
the same way — or a probe that has silently stopped measuring anything still reads as green.

Asserting a *non*-event ("this must not happen") needs an explicit `waitForTimeout` — there's no
event to poll on, which is the point. Everything else should be `expect.poll`.

Assertions that only a browser can make, and that are worth reaching for: state still holding after
`page.reload()` (proves it's server-side, not just local), and `context.close()` mid-gesture
(proves the server cleans up after a dropped client).

Ports: :8080 and :5173, shared with other sessions in this checkout — ask before killing anything
sitting on them. Runs also write to the shared `web/.e2e-data/longtable.db`.
