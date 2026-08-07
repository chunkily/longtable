---
title: The two flaky e2e specs
created: 2026-08-05
status: done
tags: [testing]
---

`token-trackers.spec.ts:144` and `asset-library.spec.ts:184` both failed intermittently under the
full run and passed in isolation. That shared symptom made them look like one problem — some
load-related timing fault — and they were nothing of the kind.

## What shipped

**Two different bugs.** Worth separating before anything else, because "passes in isolation" is
what made them look alike, and it's a symptom that means almost nothing on its own.

**The room-creation one was accumulated state.** `web/.e2e-data/longtable.db` was never reset, and
had reached **1031 rooms**. The home page lists every room, and the create-room form sits under
that list.

**Correction, 2026-08-07 — the trigger was right and the mechanism was wrong.** This originally
said the click was landing where the button had just been, as the list grew under it. It isn't
that. Filling the form *before hydration finishes* loses the values: Svelte reconciles the inputs
back to their initial `$state('')`, the click then submits an empty form, `required` blocks it,
and the page simply stays put. The failure snapshot that settled it shows all three fields empty
with the previously-created room listed above them.

Page size was still the trigger — 1031 rooms took long enough to hydrate that the race was lost
regularly, and wiping the database shrank the window rather than closing it. So the fix below is
real but incomplete, and this can come back on any page that grows. The tell is a form that
submitted nothing rather than a click that missed: check whether the fields are empty in the
snapshot before assuming a coordinate problem. `page.waitForLoadState('networkidle')` after
`goto('/')` is the reliable guard when a spec fills a form immediately after loading.
`e2e/run-backend.mjs` now wipes the database at the start of every run — at the start rather than
the end, so a failed run's data is still there to look at. `LONGTABLE_E2E_KEEP_DB=1` opts out. A
full run now leaves 72 rooms instead of adding to a pile.

**The asset one was a real behaviour disagreement, hidden by a negative assertion.** The test
asserted `getByText('map-again')` had count 0 — a condition already true of the list *as it stood
before the upload landed*. So it passed whenever it won the race against the refresh, and failed
when it lost. Underneath, re-adding identical bytes really does rename the entry: `AddAssetToRoom`
overwrites a field when the upload supplies one, and the assets page always supplies a name
because it defaults the box to the filename. The database proved it — after a *passing* run, the
row was still called `map-again`.

That behaviour was kept rather than fixed: an upload is a statement about what the image is called
now, and renaming already has its own path. The test now asserts the rename, and the credit
surviving alongside it — the credit is only sent when someone types one, so the same rule produces
the opposite outcome for the two fields, which is the clearest demonstration of it there is.

**The ordering lesson is the transferable part.** A negative assertion (`toHaveCount(0)`,
`toBeHidden`) placed where the page hasn't finished updating will pass for the wrong reason and
look stable while doing it. Assert something positive first — its arrival is the proof the update
landed — and only then assert the absence. The test does that now, and says why in a comment,
because the failure mode is invisible in a green run.

Verified with three consecutive full runs at 71/71 after the change.
