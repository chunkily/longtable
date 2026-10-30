---
title: Specs failing before they start, on a slow room creation
created: 2026-10-30
status: done
tags: [testing]
---

The longest-running flake in the suite: a spec fails inside `createRoom`, before it has done
anything of its own, because the room page didn't arrive within the 5s `expect` timeout. It came
and went across a full run and never reproduced alone. Reused room names were suspected — every
spec's room used to be called something similar — and ruled out by renaming them; the failures
continued.

## What shipped

Timing probes in `store.CreateRoom` across two full runs (2026-10-30, five workers on ten cores,
175 room creations each) found the answer: median about 200ms, 95th percentile about 1.5s, slowest
3.9s and 3.5s — and in every slow case nearly all the time was **the bcrypt hash of the GM
password**, with the database wait under 5ms. bcrypt at the default cost is the one CPU-heavy thing
the server does, and five workers each driving two or three Chromium contexts painting canvases
leave it waiting for a core. An earlier run had measured 5.8s, which is a failure. Nothing was
wrong with Longtable; the slack under the default timeout was thinner than it looked.

- `createRoom`'s wait for the room page and `joinAsGM`'s wait after signing in — the two helpers
  that wait on a hash — now take `HASH_TIMEOUT` (20s), with the measurements in its doc comment.
- **Not** a cheaper hash in the test build, so the suite keeps exercising the cost that ships, and
  **not** fewer workers, which `flaky-e2e-under-load` already found made things worse.
- The probes were removed. Anything else that waits on a hash — a spec changing the GM password, or
  signing in with it inline — is the place to look if a similar failure turns up elsewhere.

Found alongside: each run now keeps its own `test-results/<run>/` and `playwright-report/<run>/`
(see `playwright.config.ts`), because a fixed output folder wiped the evidence of a flake the next
time anyone ran a single spec.

Two other things turned up across the same runs, and neither is about the hash:

- **A run where the laptop went into Modern Standby** for six minutes partway through failed five
  consecutive specs, each "taking" 6.2 minutes against a 60s limit — the runner's own timer was
  frozen. Windows' System log (Kernel-Power 506/507) is the way to tell this apart from a real
  stall. Run folders are now named in local time so they line up with that log.
- **`asset-library.spec.ts`'s alignment-zoom test measured the preview before the map had
  decoded**: `naturalWidth` 0, an expected grid size of Infinity, and the overlay still at its
  unscaled default. It now waits for the image and for the overlay to follow it. This is the same
  test [e2e-flakes](e2e-flakes.md) first named in August.
