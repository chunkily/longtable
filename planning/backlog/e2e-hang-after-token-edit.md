---
title: Two e2e failures survive the vite fix, together about one run in three
created: 2026-08-07
status: open
tags: [testing]
---

Measured over 18 full runs after the vite cause was removed: 1-2 runs in 6 fail, and it is **not
one bug**. Two symptoms, told apart by how long the run takes:

- **The hang** (below) — the run takes ~1.6m instead of ~36s, because one test sits there for its
  whole timeout while everything else finishes normally.
- **`token-slide.spec.ts:142`** ("a token someone else moves slides rather than jumping") — fails
  in a *normal-length* run, on `expect(await transit.stop()).toBe(true)`. That assertion samples a
  0.22s slide animation (`TOKEN_MOVE_SECONDS`) by polling for ink at the halfway square, so it is
  asking to catch a moving thing in the act. A missed sample is a plausible cause and would make it
  a test weakness rather than a product bug, but that hasn't been confirmed — confirm before
  assuming, since the whole point of the test is that the token really does travel.

The rest of this item is about the hang, which is the better-understood of the two.

## The hang

`token-trackers.spec.ts:404` ("the wheel steps a focused tracker") fails roughly one run in six,
always the same way: it saves the edit dialog and then waits forever for the tracker box to appear.

```
Test timeout of 60000ms exceeded.
Error: locator.fill: Test ended.
  - waiting for getByRole('region', { name: 'Selected token' }).first().getByLabel('HP current value')
```

The box is labelled from the tracker's own label, so "HP current value" never existing means the
label `HP` never came back from the server. `save()` already waits for the dialog to close, so the
submit happened.

**This is what is left of the e2e flakiness after the real cause was fixed.** Serving the suite
from the Go binary instead of vite removed the systematic failure — see
[e2e-served-by-dev-server](e2e-served-by-dev-server.md) — and took a cold-cache run from 14
failures to 0. This is a different, rarer thing underneath it, and it was being hidden by the
louder one.

## What's been ruled out

Worth recording, because each of these is the obvious guess:

- **Not load or contention.** The intuitive story — 14 workers on one box, everything 3x slower,
  a long test running out of budget — is wrong, and the evidence is backwards from it. Capping
  workers made it *more* frequent: 3 failing runs in 6 at `workers: 4`, against 1 in 6 at the
  default 14. A failing run is also exactly one timeout longer than a passing one (1.8m vs 52s),
  so the rest of the suite finishes at normal speed while this single test sits there. Nothing is
  slow; one thing is stuck.
- **Not the test's own budget.** Raising the timeout from 30s to 60s changed nothing but the
  number in the message.
- **Not the dialog seeding its fields late.** `load()` runs from `onOpenChange` when the dialog
  opens, before the content renders, so the `fill` can't be overwritten by it afterwards.
- **Not another spec's network interception.** `page.routeWebSocket` and `page.route` (in
  `reconnect.spec.ts` and `drawing-optimistic.spec.ts`) are page-scoped and can't reach another
  test's client.
- **Not reproducible alone.** 12 consecutive runs of just this test pass. It needs the rest of the
  suite running, which is what makes the shared server and the single SQLite connection the
  interesting suspects.

## Where to look next

The token update goes over the socket, so the hang means the broadcast never arrived. Two threads
worth pulling:

- **`RoomClient.send()` returns whether the socket was open, and `updateToken` doesn't check it.**
  A command issued while the socket is down is dropped silently, with nothing on screen to say so
  — which would look exactly like this. Whether a socket could be down at that moment is the
  question; if it can, that's a product bug rather than a test one, and the fix is the same
  optimistic-rejection path `draw.create` already has.
- **The store runs on `SetMaxOpenConns(1)`.** Every request in the process shares one SQLite
  connection, so anything holding it holds up every room. The asset specs re-encode images on the
  same server; whether any of that can block a connection long enough to matter hasn't been
  measured.

The next person to hit this should run with `--trace=retain-on-failure`, then open the trace and
check the network/console panes for the failing page: whether the `token.update` frame was ever
sent, and whether an `error` event came back, separates the two threads above immediately. Capture
the artifact before the next run — Playwright wipes `test-results/` on start.
