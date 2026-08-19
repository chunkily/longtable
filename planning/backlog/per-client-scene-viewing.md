---
title: Per-client scene viewing (multi-scene support)
created: 2026-08-14
status: done
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

## What shipped

`scene.view` — everyone's, answered with `scene.viewed` to the sender alone — beside the
`scene.setActive` that was already there, now reached from `Move everyone` and GM-only. The
Scenes dialog is everyone's menu entry, with `View` on every row a Player can use and the making,
remapping, deleting and moving left off for them. Each row says which of the two it is: a
`Viewing` badge, a `The table is here` badge, or both.

**Making a scene now moves the GM who made it, and nobody else.** That wasn't in the original
description and is the thing that makes prep work actually work: filling in a form about a new map
and being left standing on the old one reads as the form having failed, and it used to be that
moving there at all dragged the party with you.

Decisions worth not rediscovering:

- **The hub keeps no per-connection viewed scene, and that's load-bearing rather than lazy.**
  Nothing server-side is scoped to where a client is looking: scene-scoped events go to the whole
  room and each client drops the ones for scenes it isn't showing (it already did — that filter
  predates this), hidden tokens are filtered by role rather than by scene, and every connection
  opens on `room.active_scene_id`. Storing it would buy nothing and would then have to be kept in
  step with every delete. `handleSceneView` answers and forgets.
- **The open question about `state.sync` answered itself.** It was already keyed off the room's
  active scene and stays that way — that *is* the "fresh join and reconnect land on the room's
  scene" decision, so the round trip the item asked for is only needed for a deliberate switch.
- **`room.active_scene_id` had to reach the client**, which it now does on `roomPayload` — so it
  rides `state.sync` and `room.updated` alike, and `scene.activated` keeps it current after that.
  Two things need it: saying which scene the table is on, and getting home.
- **Deleting a scene someone is privately standing on is handled on the client**, which sends
  itself back to `activeSceneId` on `scene.deleted` for the scene it is showing. The server's
  refusal still covers the room's own scene, which is the one with no foreign key behind it. Doing
  the recovery server-side would have meant tracking viewed scenes for this alone.
- **The row could not hold five text buttons.** Name, `View`, `Move everyone here`, `Replace map`
  and `Delete` wrapped every row in a 448px dialog — the first version of this shipped like that
  and was wrong. Now: the two navigation actions stay words, the two maintenance ones are icons
  keeping the aria-labels they had, the button lost its `here` (the row names the scene), the name
  has a floor under it so it can't be squeezed to `The…`, and this one dialog is `sm:max-w-lg`
  rather than the default. An overflow menu was the obvious answer and is unavailable: a popover
  inside a dialog comes out unpositioned and under the overlay (`seats-dialog.svelte`).
- **Armed, the delete icon gives up the icon and says `Really delete?`.** A destructive colour is
  the whole signal otherwise, which is no signal to anyone who can't see the difference — and this
  is the control that takes a scene's tokens, fog and drawings with it. The row wraps while it is
  armed, which is a second of layout against a click nobody can take back.
- **Being away from the table is said in the rail**, not on the map. Without it, prepping looks
  exactly like the room being on the new scene, and a Player who went for a look has nothing
  saying why the map stopped changing. `Go there` beside it is the way back that doesn't need the
  menu.
- The **initiative tracker** stays visible from every scene, untouched: it belongs to the room,
  it has no idea which scene a fight is on, and nothing about looking elsewhere changes whose turn
  it is.
- **Undo/redo resets on a switch**, through the same `resetAfterSync()` that `scene.activated`
  already ran — the actions refer to drawings on a scene that is no longer on screen. Token
  selection needed nothing: it's derived from `tokens`, so a selection whose token isn't in the
  scene reads as nothing selected, and comes back if you return.

## Related user stories

- [room-member-switch-viewed-scene](../user-stories/room-member-switch-viewed-scene.md)
- [gm-move-room-to-scene](../user-stories/gm-move-room-to-scene.md)
