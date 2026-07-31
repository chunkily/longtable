---
title: Scene management (switch, delete, replace map)
created: 2026-07-29
tags: [scenes, gameplay]
---

Round out scene management. Today a room can have multiple scenes, but there's no way to reach
any of them except the one just created — `scene.create` auto-activates immediately because
there's no scene-switcher UI (`internal/ws/hub.go:508-543`). There's also no way to delete a
scene or replace its map image after creation (`internal/store/scene.go` only has `CreateScene`
and `GetScene`).

`GridOffsetX`/`GridOffsetY` were found dead (stored and sent to the client but never applied in
the grid/fog/token rendering math) — rather than wiring those scene-level fields up, alignment is
now handled at asset upload time instead, baked directly into the image's pixels. See
[map-asset-grid-offset-padding](../open/map-asset-grid-offset-padding.md); the scene-level fields
become unnecessary under that approach.

Deleting a scene has to take its fog with it
([gm-delete-scene](../../user-stories/gm-delete-scene.md)), and
[fog-of-war-controls](fog-of-war-controls.md) already shipped `store.ClearFog(sceneID)`
for exactly that delete — no need to write the query again.

## What shipped

A **Scenes** dialog next to *New scene*, listing every scene in the room with three actions per
row: switch to it, replace its map, delete it. `state.sync` now carries `scenes`, and
`scene.created` / `scene.updated` / `scene.deleted` keep that list current for everyone.

Decisions worth not rediscovering:

- **Only the room's first scene auto-activates now.** Creating the *second* scene mid-session is
  prep work; yanking the party off the map they're standing on to look at an empty one is not
  what a GM meant by "New scene". The old unconditional activation existed purely because
  activation was the only way to ever reach a scene again — the thing this item removed. Two e2e
  specs and two hub tests asserted the old behaviour and were updated deliberately, not
  worked around.
- **Replacing a map broadcasts `scene.updated`, carrying only the scene.** Sending
  `scene.activated` instead would have been less code and quietly wrong: it carries the full
  picture, so clients treat it as a scene change and run `resetAfterSync()`, discarding undo
  history and any in-flight gesture. Preserving what's on the scene is the entire point of
  replacing a map rather than building a new one.
- **The active scene can't be deleted, and that's a refusal rather than a repair.**
  `room.active_scene_id` is a plain column with no foreign key, so deleting the scene it points
  at leaves every client staring at something the server can no longer load. Picking a
  replacement automatically would be guessing at what the GM wanted; switching away first is one
  click.
- **Bounds travel with the image on replace**, the same way they do at creation, and for the same
  reason: they describe the map, so keeping the old ones stretches the new art to the shape of
  the art it replaced. Fog cells are grid coordinates and stay valid either way.
- **Delete confirms in place** — the row's own button arms to "Really delete?" — because a dialog
  can't open over the dialog it would be confirming, and a scene takes its tokens, fog and
  drawings with it.

Two corrections to what's written above this section:

- The note about reusing `store.ClearFog(sceneID)` for the delete turned out to be unnecessary.
  `token`, `fog_cell` and `drawing` all have `ON DELETE CASCADE` on `scene_id`, and the
  connection sets `foreign_keys(ON)`, so a bare `DELETE FROM scene` takes all three with it.
  `DeleteScene` has a test that would fail if that pragma were ever lost, since nothing else in
  the codebase depends on cascade behaviour.
- `GridOffsetX`/`GridOffsetY` are still dead, and still sent to the client. This item didn't
  touch them, as planned.

Testing note for whoever adds to `e2e/scene-management.spec.ts`: uploads come from
`e2e/fixtures/`, which exists because of a mistake made here. The fixture started as inline
base64 hand-edited from another spec's literal to get "different" pixels, which produced a
corrupt PNG — rejected by `imageproc`'s content sniffing with a 400, and the symptom is an asset
picker that just stays empty, with no server-side log and an error toast that expires before a 5s
locator timeout does. Files can't be corrupted that way, and uploading by path keeps a fixture's
name tied to its bytes, which matters more than it looks: see `e2e/fixtures/README.md`.

## Related user stories

- [gm-switch-active-scene](../../user-stories/gm-switch-active-scene.md)
- [gm-delete-scene](../../user-stories/gm-delete-scene.md)
- [gm-replace-scene-map](../../user-stories/gm-replace-scene-map.md)
