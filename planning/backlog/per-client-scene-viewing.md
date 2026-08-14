---
title: Per-client scene viewing (multi-scene support)
created: 2026-08-14
status: open
tags: [scenes, gameplay]
story: room-member-switch-viewed-scene
---

Today a room has exactly one scene "on screen" for everyone: `room.active_scene_id`, and switching
it — `handleSceneSetActive` in `internal/ws/hub.go:1772` — is GM-only and broadcasts
`scene.activated` to the whole room (`broadcastSceneActivated`, `internal/ws/hub.go:1800`),
moving every connected client at once. There's no way for one Room Member to look at a different
scene than the rest of the table, and no way for the GM to prep a new scene privately before
unveiling it — switching to it *is* unveiling it.

This item splits "what scene am I looking at" from "what scene is the room's default", so:

- Every Room Member gets their own locally-viewed scene, independent of everyone else's.
- A fresh join or a reconnect lands on the room's active scene (`room.active_scene_id`) — the
  same default as today, just no longer the only option.
- Any Room Member — GM or Player — can navigate to any other scene in the room afterward, without
  moving anyone else. This is deliberate, not a gap to close later: the table is trusted
  ([ADR-0007](../decisions/0007-the-table-is-trusted.md)), and a Player who wanders off to look at
  a scene ahead of schedule is spoiling themselves, which is allowed.
- The GM additionally gets a "Move everyone here" action — a new command, distinct from privately
  browsing to a scene — that sets `room.active_scene_id` and force-updates every connected
  client's local view to match, the same way `scene.setActive` does today. This is how the GM
  actually reveals a prepped scene to the table.

## Design decisions already made

- **New joins and reconnects default to the room's active scene**, not wherever that Room Member
  last happened to be looking. Simpler, and matches today's single-scene behaviour for anyone who
  never wanders off it.
- **The GM's own view is independent too.** Switching the GM's local view never moves anyone —
  only the explicit "Move everyone here" action does. This is what makes prepping a scene ahead of
  time possible at all: today, activating a second scene *is* the reveal.

## Open questions for whoever picks this up

- `state.sync` currently carries one scene's worth of state per connection
  (`sceneStatePayload` in `internal/ws/state.go`), keyed off `room.active_scene_id`. This needs to
  become keyed off *that connection's* locally-viewed scene instead, and switching a client's own
  view needs a request/response round trip (a new command, not a broadcast) to fetch that scene's
  tokens/fog/drawings.
- Does the initiative tracker (room-scoped, not scene-scoped, deliberately — see "Where things
  stand" in `CLAUDE.md`) stay visible from every scene, or only the one it's "about"? It has no
  concept of which scene a fight is happening on today.
- Undo/redo is per-session already (`web/src/lib/room.svelte.ts`) — does navigating away from a
  scene and back reset it, the same way `resetAfterSync()` does on a room-wide `scene.activated`
  today?

## Related user stories

- [room-member-switch-viewed-scene](../user-stories/room-member-switch-viewed-scene.md)
- [gm-move-room-to-scene](../user-stories/gm-move-room-to-scene.md)
