---
title: Highlight selected color
created: 2026-07-29
status: done
tags: [drawing, ui]
story: room-member-selected-color-highlight
---

Highlight selected color with a light blue focus so that if black is selected it is visible.

## What shipped

A light blue outline on the selected swatch, drawn *outside* the element via `outline` with an
offset rather than as a border or an overlay. That matters for two reasons beyond looks: an
outline takes no space inside the swatch, so it never covers the colour it is marking, and it
can't end up between the swatch and the pointer the way a stacked element does.

The selected state also rides on `aria-pressed`, so it is reported to screen readers rather than
existing only as pixels — and, usefully, that is what makes it testable at all.
