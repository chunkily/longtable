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
[map-asset-grid-offset-padding](map-asset-grid-offset-padding.md); the scene-level fields become
unnecessary under that approach.

Deleting a scene has to take its fog with it
([gm-delete-scene](../../user-stories/gm-delete-scene.md)), and
[fog-of-war-controls](../done/fog-of-war-controls.md) already shipped `store.ClearFog(sceneID)`
for exactly that delete — no need to write the query again.

## Related user stories

- [gm-switch-active-scene](../../user-stories/gm-switch-active-scene.md)
- [gm-delete-scene](../../user-stories/gm-delete-scene.md)
- [gm-replace-scene-map](../../user-stories/gm-replace-scene-map.md)
