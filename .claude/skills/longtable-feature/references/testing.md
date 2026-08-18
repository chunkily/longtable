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

- `readEnvelope(t)` reads one envelope with a 2s timeout and fails the test on error. It **skips
  presence chatter**: a second client connecting announces itself to the first, and a test about
  fog or tokens shouldn't have to know that. `readAnyEnvelope` is the unfiltered primitive and
  `readPresence` its mirror, for the presence tests themselves.
- `expectNoMessage(t, d)` proves a recipient was *deliberately* skipped — the assertion for hidden
  tokens, where "nothing arrived" is the whole point. It ignores presence for the same reason;
  `expectNoPresence` is the opposite assertion.

  **Both leave the connection unusable.** coder/websocket tears down a connection whose read
  context is cancelled, so the deadline expiring — the success case — closes it. Anything they're
  called on has to be finished with. For a "nothing happened" check *mid-test*, send something and
  assert it comes straight back instead (`assertNoPresenceYet` in `presence_test.go`): whatever
  the server wrongly broadcast is queued ahead of the echo, so it's caught without a timeout.
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
disconnects. `npx playwright test` builds the frontend, builds the Go binary that embeds it, and
runs that one binary on **:8080** (see `playwright.config.ts` and `e2e/run-app.mjs`).

**The suite does not run against `npm run dev`, and putting it back would reintroduce a bug.**
Vite discovers dependencies lazily and force-reloads every connected client when it re-optimizes
("optimized dependencies changed. reloading"), which loses whatever a test was mid-way through.
On a cold `node_modules/.vite` that cost exactly one failure per worker — 14 workers, 14
failures, each a worker's *first* test — and looked like flakiness because every run after the
first passed. Serving the built SPA from the binary removes the whole class, tests the artifact a
Host actually runs (including the SPA fallback and same-origin `/api`/`/ws`), and is *faster*:
~35s against ~50s. See [e2e-served-by-dev-server](../../../../planning/backlog/e2e-served-by-dev-server.md).

To iterate on a failing spec with HMR, start `npm run dev` and a backend yourself and point
Playwright at :5173 by hand — just don't make it the default again.

Reading the canvas is the standard trick — count opaque pixels on a layer, either in a small box
around a point (`inkAt`) or across the whole layer when the shape moves (`pingInk`, `measureInk`).
Remember `devicePixelRatio` when indexing into `getImageData`.

**Multi-touch needs raw CDP.** `page.touchscreen` is single-touch, so a pinch can't be driven from
the ordinary API. `pinch-zoom.spec.ts` sends `Input.dispatchTouchEvent` over a
`context.newCDPSession(page)` with two `touchPoints`, and it's the only spec that does — it is
Chromium-only, which is fine here because the suite runs Chromium alone. Two things it gets wrong
easily: the context needs `hasTouch: true` or the browser never dispatches touch at all and the
calls land silently, and the move has to be sent in several steps rather than one jump, since a
gesture handler that accumulates a ratio against the previous sample is only exercised by more
than one sample.

That spec is deliberately thin, because the arithmetic behind the gesture lives in
`web/src/lib/pinch.ts` with unit tests. The rule generalises: when a gesture is awkward to drive,
put the maths in a plain module so the part that has to be *correct* is the part that's cheap to
test, and leave the spec to prove the wiring.

**Everything that isn't a spec lives in `e2e/fixtures/`**, so the `e2e/` folder itself is exactly
the list of tests. Its README is the thing to read before writing a new spec — three shared
modules, the first two newer than most of the specs:

- **`e2e/fixtures/table.ts`** — the `test` fixture. `{ table }` gives a room with a scene and a GM
  (`table.gm`), plus `table.join(name)` for each additional person on their own context. **Use it
  rather than building contexts by hand**: its teardown runs after a failure, and the
  `await gm.context.close()` on a test's last line does not — a failing test used to leak its
  contexts, each keeping a live socket and a connected participant for the rest of the run.
  `test.use({ scene: false })` for a room with no scene. **It leaves the Scenes dialog open over
  the map**, which is a trap worth knowing before it costs you an afternoon: creating a scene is a
  mode of that dialog, so it returns to the scene list rather than closing, and
  `expect(canvas).toBeVisible()` says nothing about what is on top of the canvas. Specs get away
  with it because their first raw-coordinate gesture lands outside the dialog and dismisses it on
  the way through; a right-click in the *middle* of the map lands on a scene list item instead, and
  looked for two rounds like a bug in the app. A spec that needs the middle of the map should
  dismiss the dialog itself, with **Escape** — `map-pan.spec.ts` has the helper.

  **Closing it in this fixture was tried and reverted.** It is the right shape of fix and it broke
  30 tests: `getByRole('button', { name: 'Close' })` resolves against several buttons sharing that
  name, picked one belonging to an already-closed dialog, and then spun on
  `element was detached from the DOM, retrying` until every test using the fixture timed out. Doing
  it properly needs a locator scoped to the Scenes dialog and a settled state to click in — worth
  doing, but as its own change with the whole suite run behind it, not as a rider on a feature.

  **When a raw-coordinate gesture behaves as though it never reached the canvas, read the page
  snapshot in `test-results/<test>/error-context.md` before theorising.** It renders what was
  actually on screen, and would have answered that one in one round instead of three.
- **`e2e/fixtures/map.ts`** — everything about the canvas: `LAYER`, `layerInk`, `inkAt`/`tokenInkAt`,
  `canvasBox`, `spawnCentre`, `dragToken`, `settleAt`, `watchInkAt`, `createToken`, `selectToken`,
  `openEditor`, `saveEditor`, `detailsPanel`, `trackerBox`. The waits in here are the ones that
  were right; the flaky specs were the copies that guessed. `createToken` returns the point the
  token landed on and waits for ink *there* rather than anywhere on the layer, and `selectToken`
  clicks until the panel names the token rather than once. `watchInkAt` samples one spot per
  animation frame, for anything transient enough that an `expect.poll` can miss its window
  entirely — and **several watches on one page are safe**: each takes its own slot, which they
  didn't before, so the second used to reset the first's result and the first `stop()` switched
  both loops off. That failed in the dangerous direction, since a spec asserting "this never
  appeared" passes when its watcher was turned off early.
- **`e2e/fixtures/room.ts`** — getting into a room and driving its chrome: `createRoom`,
  `selectTool`, the menu, the join flow.

Import them, don't re-declare them. They used to be
copy-pasted per spec, which stopped working the moment the full-bleed layout put tools behind a
family and scene actions behind a menu: "click the button called X" became two or three steps, in
twenty files.

- `selectTool(page, name)` — opens the tool's family first if it has one (`TOOL_FAMILY` maps
  variant → family), then clicks it and waits for `bg-primary`. Tool handlers are rebound in a
  Svelte effect, so a drag sent in the same tick as the click can land on the previous tool.
  Neither family nor variant buttons toggle, so this leaves the requested tool selected whatever
  was selected before.
- `selectToolFamily(page, family)` — for the controls that sit on a family's strip but aren't
  tools, like fog's `Reveal all` and `Hide all`.
- `mapGestureOrigin(page)` — the canvas corner plus `TOOLBAR_CLEARANCE_Y`. The toolbar floats over
  the map's top-left corner now, so a gesture starting at the true origin lands on a button
  instead of the map. **A spec that also probes pixels has to add `TOOLBAR_CLEARANCE_Y` back on**,
  because the canvas's own buffer still starts at its true corner — this is the single most common
  way to update a drawing spec wrongly and have it fail with "expected > 0, received 0".
  Canvas-relative pixels still double as world coordinates, because a fresh scene starts at the
  identity transform; don't pan or zoom in a spec that relies on that.
- `openRoomMenu` / `openNewSceneDialog` / `openScenesDialog` / `openAssetsPage` — Scenes and
  Assets live in the menu behind the third icon at the foot of the side panel. **New scene is not
  a menu entry**: it's a mode of the Scenes dialog, so `openNewSceneDialog` goes through
  `openScenesDialog` first. Creating a scene closes the whole dialog rather than returning to the
  list, which is what lets a spec click the canvas straight afterwards.
- `createRoom(page, roomName)` — the home page asks one question at a time, so creating a room is a
  click on `Create a room`, a wait for the form, three fills and a submit. Returns the room code.
  **Never spell this out in a spec.** Two of the steps are waits with reasons: `networkidle` before
  the click, because a click landing before hydration does nothing and leaves you waiting on a form
  that never opened, and the form's own appearance afterwards, which is what proves hydration
  finished before anything gets filled in.
- `joinAsNewPlayer(page, name)` / `takeSeat(page, seatName)` / `joinAsGM(page, name, password)` /
  `openSeatPicker(page)` — the pre-join screen asks which side of the screen you're on before it
  asks anything else, so getting into a room is never one `fill` and one click any more. Filling
  `Your name` straight after `goto('/r/…')` finds no such field: it's two steps in on the Player
  path (Player → I'm new here) and one on the GM's. `openSeatPicker` waits for the `I'm new here`
  slot, which is the one control that renders whatever the seat list comes back with — that wait
  matters for the specs asserting a seat is *absent*, since an unanswered fetch looks exactly like
  a table nobody has sat down at.

Two more that bite once a spec drives *two* browsers:

- **Each page needs its own canvas box.** This used to be load-bearing for a different reason: a
  GM's toolbar carried scene, token and fog controls a Player's didn't, so the two canvases were
  neither the same size nor at the same page offset, and reusing one page's box for the other's
  mouse landed the drag several rows off. The full-bleed layout removed that particular hazard —
  the toolbar floats *over* the map rather than pushing it down, so both roles now get a canvas of
  the same size at the same offset.

  Keep re-reading the box per page anyway. It costs nothing, it's what
  `token-trackers.spec.ts`'s `selectToken` and `token-move-undo.spec.ts` already do, and the
  property it relies on — that the two layouts stay identical across roles — is exactly the kind
  of thing the next layout change breaks silently. The old symptom is worth remembering: a click
  that selects nothing, with no error anywhere.
- **A second browser *context* is not a second tab, and for seats that's the whole test.** Tabs
  share `localStorage`, so a second tab reuses the first one's session token and can never
  exercise the seat picker or "two devices, one seat" — it just proves the two-tabs case again.
  `browser.newContext()` gets a device that has genuinely never been here. `seats.spec.ts` is
  built entirely on that distinction and says so at the top.
- **Don't grab a token that is still sliding.** A move made in another browser arrives as a 0.22s
  tween (`TOKEN_MOVE_SECONDS`), and ink shows up under a probe partway through it — so polling for
  the token at its destination returns *before* it has settled. A drag started in that window
  fights the tween and leaves the token where it was. Poll, then wait out the slide; that spec's
  `settleAt` does both.

`selectTool` used **not** to work for either fog tool, both of which relabelled themselves
(`Reveal fog` → `Painting fog…`) rather than only restyling, so the locator stopped matching the
moment the tool went active and the wait could never pass. Since the full-bleed layout they're
icons on the fog family's strip with stable names, and the shared helper handles them like any
other variant — `fog-controls.spec.ts` and `drawing-right-click.spec.ts` keep one-line aliases
over it and a note saying why they no longer need their own.

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
- **A failing run leaves a trace behind**, which is the fastest way into anything that only fails
  under a loaded suite. `trace: 'retain-on-failure'` in `playwright.config.ts` records every test
  and keeps the recording only for the ones that fail:

  ```bash
  npx playwright show-trace web/test-results/<test-dir>/trace.zip
  ```

  It plays back per-action screenshots with the DOM, network and console at each step — which for
  a canvas assertion is the only thing that answers "had it painted yet?". `retain-on-failure`
  rather than `on-first-retry` because `retries` is 0 and should stay 0: a retried flake reads as
  a pass. There is deliberately no `video`; see the comment in the config for why it would
  silently record nothing here.
- **A visible `<canvas>` is not a painted one.** `expect(canvas).toBeVisible()` resolves as soon as
  the element has a box, which is at least a frame before Konva has drawn into it, and a re-render
  (a scheme change, a scene switch) empties a layer again on the way through. `theme.spec.ts`
  waits for a painted pixel rather than reading straight after visibility — and the reason that
  matters is not only the loud failure ("no grid lines drawn") but the quiet one: an unpainted
  pixel reads as transparent *black*, which satisfies every "this should be dark" assertion in
  that file. A probe that samples too early can pass for the worst possible reason.
- **Every connection writes a line into the chat log**, so presence is now noise in two places at
  once. `readEnvelope` skips both via `isPresenceNoise`; `saidByPeople` and `saidInSync` strip the
  room's own lines out of a stored log or a `state.sync` payload, which is what a test asserting
  "the log is empty" actually means. A test that *wants* them uses `readSystemLine`.
- **A departure is announced by a timer**, half a minute after the socket closed. `hurryDepartures`
  shortens it for the tests that want to watch someone leave; everything else should leave the
  production value alone, since a timer firing mid-test turns unrelated assertions into a race.
  The e2e harness writes a `longtable.toml` holding `departure_grace = "2s"` and starts the
  server with `-config` pointing at it — long enough that a spec reloading a page doesn't
  trip a spurious departure, short enough for a presence assertion to outlast it.
- **`toBeVisible()` says nothing about opacity**, so a popup whose enter animation never finishes
  passes every locator assertion in the suite while being invisible to a person. Worth knowing
  before reading a hand-inspection of one: a page in a browser pane that isn't being displayed
  doesn't composite, so its CSS animations sit frozen at time 0 and every measurement taken there
  is of the *first frame*. Both popovers measured 0.95 scale and opacity 0 that way and are fine
  under Playwright, which composites.

Both of those produced passing tests that asserted nothing. Pair a "this must not happen"
assertion with a positive control on the *same* probe — the gesture that *should* work, checked
the same way — or a probe that has silently stopped measuring anything still reads as green.

Asserting a *non*-event ("this must not happen") needs an explicit `waitForTimeout` — there's no
event to poll on, which is the point. Everything else should be `expect.poll`.

**`page.mouse.click(x, y)` skips every actionability check `locator.click()` makes**, including
"is anything on top of this point". A dialog closed a moment earlier is still covering the middle
of the canvas while its exit animation runs, and still takes the click — so the canvas never hears
it and the feature looks broken. Wait for the dialog to be *gone*
(`await expect(page.getByRole('button', { name: 'Create token' })).toBeHidden()`), not just for
whatever the dialog produced, before any raw mouse coordinates. `token-selection.spec.ts`'s
`createToken` waits on both that and the token arriving over the socket, and says why.

Assertions that only a browser can make, and that are worth reaching for: state still holding after
`page.reload()` (proves it's server-side, not just local), and `context.close()` mid-gesture
(proves the server cleans up after a dropped client).

**Simulating an insecure context.** Playwright drives `localhost`, which is always a secure
context, so the browser APIs that only exist in one are always present — and the bugs that come
from assuming them are invisible to the whole suite. `insecure-context.spec.ts` fakes it with
`page.addInitScript` taking the API away before any page script runs, which is the shape every
client on a LAN address actually sees. Two things make that spec worth copying rather than
rewriting: it asserts the API really is gone before testing anything (a stub that quietly stopped
applying would leave a test that passes while checking nothing), and it reloads afterwards, because
what matters isn't that the client stopped throwing but that the *server accepted* what the
fallback produced.

Port: **:8080** only, since vite is no longer in the picture — still shared with other sessions in
this checkout, so ask before killing anything sitting on it. Runs write to
`web/.e2e-data/longtable.db`, **which is wiped at the start of every run** (`e2e/run-app.mjs`), so
what's in there afterwards is the last run's and nobody else's. Set `LONGTABLE_E2E_KEEP_DB=1` to
append to the previous run instead.

One known hang survives all of this, about one run in six:
[e2e-hang-after-token-edit](../../../../planning/backlog/e2e-hang-after-token-edit.md). Before
calling anything else flaky, read that item — it records what has already been ruled out, and
capping workers is one of the things that made it *worse*.

**Run `npx playwright test` from inside `web/`, not the repo root.** From the root, npm's
upward `node_modules` search can resolve a different `@playwright/test` than the one `web/`
depends on, and the failure it produces is misleading: `"Playwright Test did not expect test()
to be called here"` / `"two different versions of @playwright/test"`, pointing at a `test(...)`
call in a spec file that is completely fine. If that error shows up, check `pwd` before doing
anything else — don't start editing the spec.

**Testing a file upload:** go through the assets page — `/r/{slug}/assets`, reachable by the
`Assets` link in the room header. The pickers in the scene and token dialogs only *pick*; nothing
uploads from inside a dialog any more. The gesture is
`page.getByLabel('Choose images to add').setInputFiles(fixture('goblin.png'))`, then filling
`Name` if the default matters, then `Add to library`. **A spec whose upload is a map has to switch
tabs first** — `getByRole('tab', { name: /^Maps/ }).click()` *before* `setInputFiles`, because the
page's tab is what decides the kind and it's read when the file is staged, not when it's added.
Clicking it afterwards moves the grid, not the staged file. This matters even when the spec
doesn't care about tabs: the scene and replace-map pickers open on Maps, so a map filed as token
art simply isn't in the grid the spec then clicks in. The regex anchor is worth copying — the tab
is named `Maps 1`, counts included, and Playwright matches names by substring unless told
otherwise. `fixture()` comes from `e2e/fixtures/images.ts`
and the images live beside it (see that folder's README). It works on a hidden
`<input type="file">` — visibility doesn't matter to Playwright the way it does to a real click —
so there's no need for a visible-input workaround.

Uploading **by path, not `{ name, mimeType, buffer }`**, is still the default, though the reason
has narrowed twice. Assets are content-addressed, so identical pixels uploaded by an unrelated
spec *in the same run* resolve to the same asset row, carrying the `filename` the bytes were
first stored under. What a spec sees on screen is the per-room `name`, a `room_asset` column
belonging to this run's room, so a fresh room always shows the name this run supplied. Assert on
the name; `filename` is the field that drifts. (This used to reach across runs too, back when the
database was never reset — it no longer does, but within one run it still holds.) Sending the real basename keeps the default name tied to the fixture anyway, which is one
less thing to spell out in a spec. The one deliberate exception — re-adding identical content
under a different name to prove dedup — passes bytes explicitly and says why.

Two rules for adding a fixture, both already paid for:

- **Encode it, don't hand-edit one.** Mutating a few base64 characters of an existing PNG to get
  "different" pixels produces a corrupt file, and the failure is genuinely hard to read: the
  upload answers 400, the server logs nothing (the request never gets past the decode), and the
  only symptom is a library that stays empty — the error toast expires before a 5s locator
  timeout does. `imageproc.Reencode` sniffs content, so nothing that isn't really an image gets
  past the handler.
- **Give it pixels no other fixture has** — a different flat colour is enough — or the
  content-addressing above collapses it onto an existing row. This includes anything uploaded by
  a manual check through the Browser pane, which talks to the same database (see
  `.claude/launch.json`'s `backend` config).

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
