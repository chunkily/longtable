---
title: E2E assertions with a margin of one frame
created: 2026-08-26
status: done
tags: [testing, e2e]
---

`token-slide.spec.ts` failed about once a full suite and passed every time it was run on its own,
for long enough that it had been noticed more than once. It is the third flake in this suite with
the same shape as the other two, and the shape is worth naming: **an assertion that depends on a
single painted frame landing in a small window.**

## What was actually happening

A token's slide is a fixed 220ms whatever the distance (`TOKEN_MOVE_SECONDS` in
`game-canvas.svelte`), so a move is about thirteen painted frames, and the easing spends most of
them near the two ends. The spec proved "it slid rather than teleporting" by watching a 20px box
on the midpoint.

Measured, by sampling the watching page every frame and recording where the ink actually was, at
four CPU throttling rates:

| CPU rate | frames painted | frames anywhere between the squares | frames within 20px of the midpoint |
| --- | --- | --- | --- |
| 1x (idle) | 33 | 4 | **1** |
| 4x | 36 | 4 | **1** |
| 10x | 21 | 2 | **0** |
| 20x | 20 | 0 | **0** |

**One frame, on an idle machine.** Not a rare race — no margin at all. Anything that cost the page
a frame at the wrong moment (another worker's browser, a GC, a slow paint) took the only evidence
the assertion had. Under any real load there was nothing left, which is why it failed in a full
suite and never alone.

Note the second column: the animation *was* rendering frames between the squares. The assertion
was not failing because the token teleported; it was failing because the frames it painted were
not the ones the probe was looking at.

## What shipped

`watchInkBetween(page, from, to)` in `web/e2e/fixtures/map.ts` — watches the whole band between
the two squares, inset by a square at each end so a token sitting still at either end never
registers. That clearance is what the midpoint probe was buying by being small, and it is kept by
geometry instead. The spec also moves six squares rather than four: a longer path doesn't buy more
frames, since the duration is fixed, but it puts a larger share of them clear of both ends.

Re-measured with the new geometry, the band sees 5–6 frames idle, 2 at 10x, and **1 even at 20x**
— where the old probe had none from 4x upward.

`watchInkAt` and `watchInkBetween` now share one sampler, and `stop()` reports the frame count as
well as the answer. That second part closes a hole in the *other* direction, found while fixing
this one: **a watch that never ran reports "never seen", which is a pass for every assertion of
the form "this must not appear".** Two such assertions were relying on that — the rubber-band
check in this same file and the "nobody else sees the ghost" check in
`token-drag-preview.spec.ts`. Both now assert `frames > 0` first, so the watcher has to prove it
was awake before its silence counts as evidence.

## The pattern, for the next one

Three flakes in this suite have now had the same cause, and it is not timing in general — it is
**evidence that exists for one frame**:

- `e2e-flakes.md`: an `expect.poll` from the test process missing a one-shot event entirely, fixed
  by sampling inside the page per frame.
- The `Discard changes` swap: a button resolved during a ~190ms window in which the layout was
  still moving, so Playwright's own stability check lost the node.
- This one: a probe small enough that only one painted frame ever satisfied it.

When an assertion is about something transient, the question to ask is not "is this fast enough?"
but **"how many frames satisfy it, and what happens when one goes missing?"** That number is
measurable — throttle the CPU with `Emulation.setCPUThrottlingRate` over CDP and count — and if
the answer is one, the test will fail eventually and only ever under load.
