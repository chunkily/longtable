---
title: Specs that pass alone and fail in a full run
created: 2026-08-09
status: open
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
