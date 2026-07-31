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

`selectTool` does **not** work for either fog tool, both of which relabel themselves (`Reveal
fog` → `Painting fog…`, `Hide fog` → `Hiding fog…`) rather than only restyling: the locator stops
matching the moment the tool goes active, so the wait can never pass. `drawing-right-click.spec.ts`
has a `selectFogTool` for the reveal tool and `fog-controls.spec.ts` a two-label version covering
both, either of which clicks the old label and waits on the new one — which is what proves the
switch happened.

Two things that quietly make a pixel assertion measure nothing:

- **Counting non-transparent pixels is blind to a GM's fog.** A GM's cover is drawn at
  `opacity: 0.35` and revealed cells are punched out at `0.35` too, so revealing takes a pixel
  from roughly alpha 89 to roughly 58 — lower, never zero, and the count comes back identical.
  Sum the alpha channel instead (`layerAlpha`). Players get `opacity: 1` and a full punch-out, so
  the same probe works against a player's view; it's the GM rendering that defeats it. Prefer
  asserting on a *player's* canvas for anything about fog for that reason — `fog-controls.spec.ts`
  does, which is what lets it assert exact equality with the fully-covered baseline after a
  re-hide, and an exact zero after a reveal-all.
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

**Run `npx playwright test` from inside `web/`, not the repo root.** From the root, npm's
upward `node_modules` search can resolve a different `@playwright/test` than the one `web/`
depends on, and the failure it produces is misleading: `"Playwright Test did not expect test()
to be called here"` / `"two different versions of @playwright/test"`, pointing at a `test(...)`
call in a spec file that is completely fine. If that error shows up, check `pwd` before doing
anything else — don't start editing the spec.

**Testing a file upload:** use `locator.setInputFiles({ name, mimeType, buffer })`. It works on a
hidden `<input type="file">` — visibility doesn't matter to Playwright the way it does to a real
click — so there's no need to reach for a visible-input workaround. Build the buffer from a real
tiny image (`image/png` encoded in Go and pasted in as a literal, or built inline in the spec);
`imageproc.Reencode` sniffs content and rejects anything that isn't a genuine image, so an
arbitrary byte string won't get past the upload handler at all.

**Fixture content has to be unique per test, not just per spec file.** The e2e database is
persistent across runs (`web/.e2e-data/longtable.db`, gitignored, never reset between
`playwright test` invocations) and content-addressed — uploading the same pixels a second time,
weeks later, in an unrelated spec, resolves to the *same* asset row under its *original*
filename. A test that asserts on a freshly-uploaded filename will flake the moment another spec
(or a manual check through the Browser pane, which talks to this same database — see
`.claude/launch.json`'s `backend` config) has ever uploaded identical bytes. Give each fixture a
genuinely distinct pixel value (a different flat colour is enough) rather than reusing a
constant across tests.

**Manual verification through the Browser pane is a different environment from a Playwright
run**, and can fail in ways that look like application bugs but aren't. If `computer{screenshot}`
reports `"the Browser pane is not displayed, so the page is not compositing frames"`, treat
everything downstream with suspicion: coordinate-based clicks refuse to run at all without a
cached screenshot, and even ref-based clicks can silently not land. Backgrounded/non-composited
tabs are also where Chrome is most likely to throttle CSS animations and `requestAnimationFrame`
— a dialog that closes (`data-state="closed"` in the DOM, confirmed) but stays fully opaque and
interactive on screen is that, not a broken exit transition. Before concluding something in the
app is wrong: reproduce it against a real `npx playwright test` run first, and if you can't check
the pane state, try reverting the suspect file with `git stash push -- <file>` and reproducing
against the *unmodified* version — if the symptom is identical, it's the environment, not the
change.

One more manual-testing trap: checking DOM state synchronously right after a JS `element.click()`
(e.g. `el.click(); document.querySelector(...)`) can run before Svelte's reactivity has flushed,
reading stale state and reporting "nothing happened" for something that actually worked a moment
later. Await a short `setTimeout` (or a subsequent tool round-trip, which has the same effect)
before checking.
