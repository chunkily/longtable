---
title: First-time display name prompt
created: 2026-07-29
status: dropped
tags: [onboarding, identity]
story: room-member-reusable-display-name
---

On opening the application for the first time on their device, the user is prompted for their display name (which they can change later). This is re-used across all rooms. Duplicate names are allowed.

"Change later" is the [options page](options-page.md), not yet built — that item covers the edit UI once this prompt has written the initial value to storage.

## Why this was dropped

Decided on 2026-08-05 along with its story,
[room-member-reusable-display-name](../user-stories/room-member-reusable-display-name.md), which
carries the reasoning.

Short version: seats mean you name yourself once per room rather than once per join, so there is
little left for a device-level name to save — and what it would have cost is the only piece of
identity that outlives a room, which [ADR-0008](../decisions/0008-seats-and-sessions.md) had just
finished arguing shouldn't exist.

Worth knowing if this is ever revived: a prompt on first open would also be the only screen in
Longtable that appears before the user has done anything, asking for something they don't need
yet. That was the weakest part of it well before seats made the rest moot.
