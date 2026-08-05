---
title: Room Member sets a reusable display name
created: 2026-07-29
status: dropped
---

As a Room Member
I want to set my display name once for my device
So that I don't have to retype it every time I join or create a different room

## Acceptance criteria

- [ ] First-time app open prompts for a display name
- [ ] That name is used as the default when joining or creating any room
- [ ] I can change my display name later
- [ ] Duplicate display names across different people are allowed

## Why this was dropped

Decided on 2026-08-05, after [ADR-0008](../decisions/0008-seats-and-sessions.md) made most of it
unnecessary and the remainder inconsistent.

The problem it solved was retyping a name on every join. Seats remove that: you type a name once
per room, on the "I'm new here" path, and every visit after that is picking your seat back up. A
group playing one campaign together types their names once, ever. What a remembered device name
would still save is a single prefill on a form that comes up a few times a year.

The stronger reason is what it would have been rather than what it would have saved. A
device-level name is the **only** thing that would carry between rooms. ADR-0008 is explicit that
nothing about a seat does, and no-accounts is the constraint the whole identity model is built
around — so a name remembered across every room on a device is a small account. Not a credential,
but a persistent cross-room identity, and the last exception standing after everything else was
made room-scoped. Keeping it would have meant defending that exception every time someone read the
model.

Nothing replaces it. Typing your name when you first sit down at a new table is the feature.

Took [first-time-display-name-prompt](../backlog/first-time-display-name-prompt.md) with it, and
thinned [options-page](../backlog/options-page.md) to the theme control alone — which is still
worth building, since [dark-mode](../backlog/dark-mode.md) needs somewhere to put its override.
