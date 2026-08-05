---
title: Room Member's right-click never draws
created: 2026-07-29
status: done
---

As a Room Member
I want right-clicking on the map to never start a map tool
So that I can use the right mouse button for other purposes without accidentally
changing the map

## Acceptance criteria

- [ ] A right mouse button press/drag never creates or previews a drawing, regardless of the active drawing tool
- [ ] The same holds for every other tool that acts on a press: the eraser erases nothing, the fog tool reveals nothing, no ping is sent, and no measurement starts
- [ ] Releasing the right button part-way through a left-button gesture doesn't end or commit it
- [ ] Left mouse button behaviour is unaffected, for every tool
- [ ] Touch input is unaffected — touch events carry no button, and must not be mistaken for a non-primary press

## Note on scope

Originally written for the drawing tools alone, which is how it was first built. Widened after
the fact: right-clicking with the eraser active still erased, and it is the same accident with
the same cause, so scoping the story to "drawing" was the mistake rather than the implementation.
