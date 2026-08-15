---
title: Room Member chooses stroke size
created: 2026-07-29
status: done
---

As a Room Member
I want to choose the stroke size for my drawings
So that I can vary line weight instead of every stroke being the same fixed width

## Acceptance criteria

- [ ] A control on the draw strip lets me pick a stroke width before drawing
- [ ] New drawings use the selected stroke width instead of the current hardcoded value
- [ ] Stroke width is stored per drawing, so different strokes on the same map can have different widths

The first criterion said "a range input" until 2026-08-15, and it shipped as three named widths
instead — a deliberate design change, made for the reasons in
[stroke-size-range-input](../backlog/stroke-size-range-input.md), not a slider nobody got round to.
The criterion was rewritten to ask for the control rather than for its shape, which is what it was
really after.
