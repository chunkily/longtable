---
title: Selection ring had a seam where its dashes met
created: 2026-08-02
status: done
tags: [tokens, ui]
---

The rotating ring around a selected token showed one pair of dashes visibly closer together than
the rest, orbiting the token as the ring turned. Reported straight after
[token-selection-highlight](token-selection-highlight.md) shipped.

## What shipped

The dash is now derived from the circumference instead of being two fixed lengths, so a whole
number of periods always closes the ring.

It was arithmetic, not taste. `SELECTION_RING_DASH` was `[5, 7]` screen pixels; a 1×1 token on a
70px grid gives a ring of radius 41, so its circumference of 257.6 fits the 12px period 21.47
times. The leftover 0.47 was just enough for one more 5px dash plus **0.6px of gap** before it
ran into the dash the pattern started with — one gap at 0.6px where every other was 7px. Larger
tokens had it too, less severely: a 2×2 worked out at 4.5px.

The fix keeps the 5:7 ink-to-gap ratio and flexes only the period:

    periods = round(circumference / targetPeriod)
    period  = circumference / periods

Two things to know if this is touched again:

- **It has to be recomputed wherever the radius or zoom changes**, which is already true because
  `renderSelection` runs on both. A dash worked out once at mount would reintroduce the seam the
  first time anyone zoomed.
- **The period is quantised, so zooming occasionally adds or drops a single dash** as the
  rounding crosses an integer. Visible only while actively zooming, and much smaller than a
  permanent seam — accepted rather than missed.

No test covers it: the arithmetic lives inline in `renderSelection` rather than in its own module,
which was a deliberate call to keep the fix proportionate to a four-line change. If it grows a
third case it should come out into `$lib` with a spec, the way `measure.ts` and `aoe.ts` did.
