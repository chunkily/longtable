---
title: Bigger tracker boxes, with a floating step control
created: 2026-08-05
status: done
tags: [tokens, ui]
---

The tracker values in the selected-token panel were a `w-10` box at `text-xs` — small enough that
reading a hit point total across a table meant leaning in, and small enough that hitting one with
a mouse took aim. Make them a large input, and give a focused box a floating increment/decrement
control so damage can be applied without arithmetic.

## What shipped

`floating-number-input.svelte`: a large centred number box that, while focused, floats a
`− [by] +` panel above itself. The by-box is empty by default and reads as 1, so the common case
costs no typing; filling it in makes each click worth that much. The three tracker slots in
`token-tracker-strip.svelte` now use it, in a fixed three-column grid.

Four things worth not rediscovering:

- **`bind:value` on a number input hands back a *number*, and `null` for an empty box** — not the
  string that was typed. The parse helper takes both because of it. Getting this wrong is
  unusually expensive to spot: the failure is a `TypeError` inside a click handler, so the button
  silently does nothing and the typed path keeps working, because that one reads
  `event.currentTarget.value` off the DOM instead. It cost a full e2e cycle to find.
- **The step buttons `preventDefault()` on mousedown** so focus never leaves the box. That is what
  holds the panel open for a second click — "the ogre takes 7, then 3" is one interaction — and it
  also stops a blur from firing a `change` on the way past, which would commit the pre-step value
  behind the step's back. A consequence worth knowing when writing tests: after using the by-box,
  it is the by-box that holds focus, so blurring the value box is a no-op.
- **A click commits immediately**, unlike typing, which still waits for the blur or the Enter. A
  click is already a finished intent; waiting would mean the first of three clicks never landed.
  `lastSent` stops the change event that follows a click from repeating what the click just said.
- **The panel overlays the token's name and the slot labels** while it is up, because it is placed
  above a box in a section only so tall. Accepted rather than worked around — it is transient, and
  every other placement covers something worse (the conditions below, the Edit button beside).
  `flip()` moves it under the box when there is no room above, which is the phone case.

The shared `Input`'s `md:text-sm` had to be overridden with an explicit `md:text-lg`; the base
component shrinks its text from `md` up, which would have undone the whole point on a desktop.

`@floating-ui/dom` is now a declared dependency. It was already present underneath `bits-ui`, and
importing it on that basis would have been a phantom dependency — fine until bits-ui changed what
it pulls in.

## Related user stories

- [gm-set-token-hp-condition](../user-stories/gm-set-token-hp-condition.md)
- [player-set-own-token-hp-condition](../user-stories/player-set-own-token-hp-condition.md)
