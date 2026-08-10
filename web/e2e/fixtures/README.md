# What the specs are built on

Everything in `e2e/` that isn't a spec lives here, so the folder above is exactly the list of
tests. Two kinds of thing, and they share a name because both are what a spec is handed before it
starts:

| File        | What it gives you                                                                                                                           |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| `table.ts`  | the Playwright fixture: `{ table }` is a room with a scene and a GM, `table.join(name)` adds a person on their own device                   |
| `map.ts`    | the canvas — ink probes, spawn maths, drags, and the token gestures (`createToken`, `selectToken`, `openEditor`)                            |
| `room.ts`   | getting into a room — `createRoom` from the home page, the three ways to join one — and the room's own chrome: tools, the menu, the dialogs |
| `images.ts` | `fixture('goblin.png')`, an absolute path to one of the PNGs beside it                                                                      |
| `*.png`     | the images themselves — see below, both rules matter                                                                                        |

**Start a new spec with `table.ts`, not with a neighbouring spec.** Roughly two thirds of the
specs predate this folder and still build their own browser contexts by hand, closing them on the
last line — which is the one line that _doesn't_ run when an assertion fails. A leaked context
keeps a live socket and a connected participant for the rest of the run, so one real failure makes
everything after it stranger. The fixture's teardown runs whatever the outcome. Copying an old
spec copies the problem.

```ts
import { expect, test } from './fixtures/table';
import { createToken, selectToken, tokenInkAt } from './fixtures/map';

test('a token can be moved', async ({ table }) => {
	const player = await table.join();
	const spawn = await createToken(table.gm.page, 'Goblin');
	// no context bookkeeping, and no teardown to forget
});
```

`test.use({ scene: false })` for a room with no scene on it.

A spec that needs a room but not the fixture's people — one that starts its own contexts for its
own reasons — should still reach for `createRoom` rather than filling the form itself. The home
page asks one question at a time now, so "create a room" is a click, a wait and three fills, and
the wait is the part that is easy to leave out and hard to debug without.

The waits in `map.ts` are the ones that turned out to be right, and the specs that were flaky were
the copies that guessed: `createToken` waits for ink _at the square the token landed on_ rather
than anywhere on the layer, and `selectToken` clicks until the panel names the token rather than
once, because Konva only fires `click` when both halves land on the same node and `renderTokens`
rebuilds every group whenever `room.tokens` changes.

# The image fixtures

Real, small PNGs for the specs that upload something. Load them with the `fixture()` helper in `images.ts`, which resolves paths against the module
rather than the working directory.

All 8x8 except `wide-map.png`, which is 40x12 because its _shape_ is what it's for: the assets
page reads a staged file's dimensions and questions whether it's really the kind the open tab
says, and only a decidedly non-square image exercises that. `gen-wide-map.go` is the program that
produced it, kept beside it so the next odd-shaped fixture doesn't start from scratch.

Two rules, both of which have already cost someone an afternoon:

**They have to be genuinely encoded images.** `imageproc.Reencode` sniffs the content and answers
400 for anything that isn't one, so a file produced by hand-editing another fixture's bytes fails
in a way that reads like an application bug: the upload 400s, the server logs nothing (the
request never gets past the decode), and the only symptom on screen is an asset picker that stays
empty — the error toast expires before a 5s locator timeout does. Encode new ones; don't edit
these.

**Every fixture needs pixels no other fixture has.** Assets are content-addressed, so two fixtures
with identical content resolve to _one_ asset row — under whichever filename got there first, in
whatever unrelated spec ran before yours. A flat colour nobody else used is enough. Current
fixtures are one flat colour each and their hashes are distinct. (`web/.e2e-data/longtable.db` is
wiped at the start of every run now, which bounds this to one run rather than to the whole history
of the checkout. It doesn't remove it: spec order within a run is enough.)

Uploading by path (`setInputFiles(fixture('goblin.png'))`) sends the file's real basename, which
keeps the name a spec asserts on (`goblin.webp`, after the WebP re-encode) tied to the bytes that
produce it. That lockstep is the point of keeping these as files. The one spec that deliberately
breaks it — re-uploading identical content under a different name to prove dedup — passes an
explicit `{ name, buffer }` instead, and says so.

To add one:

```go
// go run this, then save stdout to a new file here
img := image.NewRGBA(image.Rect(0, 0, 8, 8))
for y := range 8 {
	for x := range 8 {
		img.Set(x, y, color.RGBA{R: 0x2f, G: 0x6b, B: 0x3a, A: 0xff}) // pick an unused colour
	}
}
png.Encode(out, img)
```
