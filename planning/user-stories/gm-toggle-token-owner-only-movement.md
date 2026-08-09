---
title: GM toggles owner-only token movement
created: 2026-07-29
status: done
---

As a GM
I want to toggle a room setting that restricts token movement to owners only
So that I can lock down movement for tables where Players shouldn't move each other's or NPC tokens, while keeping it open by default for casual play

## Acceptance criteria

- [ ] The room has an "owner-only movement" setting that is off by default, matching current behavior
- [ ] When off, any Room Member can move any token (current behavior, unchanged)
- [ ] When on, a Player can only move tokens they own
- [ ] The GM can always move any token, regardless of the setting
- [ ] The GM can toggle this setting at any time

## Verified 2026-08-09

Every criterion holds, against `mayMoveToken` in `internal/ws/hub.go` and covered end to end by
`web/e2e/movement-lock.spec.ts` with two browser contexts.

The last criterion is the one that needed more than a database column: "at any time" means
mid-session, so the change broadcasts as `room.updated` and the canvas re-renders its tokens on
it. An implementation that only read the setting at join would satisfy every other criterion here
and fail this one in the only way that matters — silently, for everyone already at the table.
