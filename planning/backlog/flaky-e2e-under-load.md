---
title: Specs that pass alone and fail in a full run
created: 2026-08-09
status: done
tags: [testing]
---

The suite is green but not reliably green. Individual specs pass in isolation and fail
intermittently when the file or the whole suite runs, which makes a red run something people learn
to re-run rather than read — the worst state for a test suite to be in, because the first real
regression it catches will be re-run too.

This is the second round of this. [e2e-flakes](e2e-flakes.md) is the first, and its lesson is the
starting point here: **"passes in isolation" is a symptom that means almost nothing on its own.**
Those two turned out to be two unrelated bugs — one accumulated state, one a genuine behaviour
disagreement hidden behind a negative assertion — and neither was the timing fault they both
looked like.

## What has actually been seen

- **`token-edit.spec.ts` — "clicking away from an edited form asks, with one dialog on screen"**
  (2026-08-09). Fails roughly one file run in five; passed 3/3 immediately afterwards, and passes
  alone every time. The gesture is `page.mouse.click(5, 5)` on a dialog overlay after a 250ms
  wait.
- **`asset-library.spec.ts:188` — "a library entry can be renamed…"**. Intermittent on Windows,
  and *not* a timing fault: the API returns `attribution: ""` where the row had a credit, and the
  server log shows a blobstore `rename … Access is denied` plus
  `UNIQUE constraint failed: asset.content_hash`. Written up in
  [player-created-tokens](player-created-tokens.md). A real bug, still open.
- **`token-trackers.spec.ts` — "the wheel steps a focused tracker…"**. Failed three full runs in a
  row on 2026-08-09, passed 3/3 in isolation, and the token came out named `TrollHP` — a fill
  landing in the wrong field, which no reading of `locator.fill` explains. Made self-checking
  (poll until the value sticks) so the next failure names its own cause; the underlying reason was
  never found.
- **`token-slide.spec.ts` — "a token someone else moves slides rather than jumping"**. One failure
  under a full run, passed on rerun and on every run since.

## A lead, and a warning about it

The dialog specs *look* like a listener-attach race: bits-ui attaches its outside-click handler a
tick after the content paints, so a click sent immediately lands before anything is listening and
the dialog stays open.

**Do not assume that is the answer — it has already been tried and made things worse.** Replacing
the fixed wait with "click again until the editor is gone" took the failure rate from about 1 run
in 5 to 3 in 4. The likely reason is instructive: the editor's fields stay in the DOM through the
exit animation, so the retry saw the editor as still present *after* a successful first click and
clicked again — landing on the warning dialog's overlay and dismissing it, which discards the edit
and takes away the very thing the test was about to assert. Whatever the fix is, it needs an
observable that distinguishes "the dialog is going" from "the dialog is here", and `isVisible()`
on a field inside it is not that.

## Worth doing first

- Turn on Playwright's `trace: 'retain-on-failure'` and read one real failure rather than
  reasoning from the symptom. Every diagnosis above that stuck came from a snapshot; every one
  that came from theory was wrong.
- Check whether these correlate with worker count. The suite runs multiple workers against **one**
  Go binary and one SQLite file, and the two most-flaky areas are the ones that touch shared
  server state.
- `web/e2e/room.ts` is where a fix belongs if the answer is a gesture helper, so every spec gets
  it at once.

## What shipped

Ten consecutive full runs, no failures. Before this the suite failed roughly one run in three to
one in five, depending on which of the causes below bit.

**Three separate causes, none of them the one everybody guesses.** "Passes alone, fails in a full
run" is what all three looked like, and load was not the mechanism in any of them — the same
lesson [e2e-flakes](e2e-flakes.md) recorded, learned again the hard way:

- **A product race in uploads, and the only one that was really about parallelism.** Storing an
  image looked before it leapt, twice: `FindAssetByHash` then insert, and `os.Stat` then rename.
  Two uploads of the same picture both looked, both found nothing, and the loser got a 500 — from
  the UNIQUE index on `asset.content_hash`, or on Windows from a rename onto a path that now
  existed (POSIX would have replaced it silently). Parallel workers upload the same fixture bytes,
  so the suite hit it; two people adding the same map to one table would have hit it in play.
  Both steps are now safe to lose, and
  `TestUploadAsset_ConcurrentIdenticalUploadsAllSucceed` fires eight simultaneous uploads and
  fails without the fix with exactly the error the flaky run showed.
- **A regression in `token-move-undo.spec.ts`, mine, from the day before.** Making token creation
  undoable meant the GM's stack was `[create, move]`, so a correctly-declined move-undo fell
  through and *deleted the token* — and whether the test passed depended on whether the deletion's
  broadcast beat the pixel probe. The token is made by the player now, leaving the GM's stack
  holding exactly the move under test.
- **Copy-pasted setup with copy-pasted waits.** Every spec carried its own `layerInk`,
  `spawnCentre`, `createToken`, and its own idea of what to wait for. `createToken` waiting for
  "any ink on the token layer" is fine until a spec has two tokens; a single click to select is
  fine until a rebuild lands between mousedown and mouseup.

**The fixtures.** `e2e/table.ts` is a Playwright fixture giving a room, a scene and a GM, with
`table.join()` for each additional person — and **teardown that runs after a failure**, which the
hand-written `await gm.context.close()` on a test's last line does not. A failing test used to
leak its browser contexts, and each leaked context kept a live socket and a connected participant
for the rest of the run: one real failure quietly made everything after it stranger. `e2e/map.ts`
holds the canvas probes and token gestures, with the *correct* waits in one place — `createToken`
returns where the token landed and waits for ink at that square, `selectToken` clicks until the
panel agrees.

Six specs are migrated (`token-edit`, `token-delete`, `token-move-undo`, `token-selection`,
`token-slide`, `token-trackers`) — the canvas-heavy ones, which is where every flake lived. The
rest still build their own contexts and are candidates for the same treatment; nothing about them
is broken, they just don't have the teardown guarantee yet.

**Two things that look like tidying and are not, for whoever migrates the rest:**

- `await player.context.close()` is sometimes the *action under test* — `token-edit`'s owner
  picker ("Bob shuts his laptop") and `measure`'s disconnect cleanup both close a context on
  purpose. A regex that strips teardown will delete the test. Ask what each close is for.
- Removing a spec's own `openRoomAsGM` without also giving it the fixture leaves it creating
  contexts and never closing them, which is worse than where it started.

**A fourth cause, found later and Go-side rather than e2e**: two unit tests failed intermittently
on Windows because a millisecond-grained `time.Now()` cannot separate rows written in one breath,
leaving their order to a tie-break that was random or absent. See
[rows-ordered-on-a-coarse-clock](rows-ordered-on-a-coarse-clock.md) — it fits this item's lesson
exactly: "passes alone, fails in a run" was not the shape, and load was not the mechanism.

**Not fixed, and still open**: the `token-trackers` hang recorded in
[e2e-hang-after-token-edit](e2e-hang-after-token-edit.md) has not been seen in these ten runs, but
ten runs is not enough to call a one-in-six flake dead. That item stays open. If it returns, its
own note is the place to start — the trace instructions there are still the right first move.
