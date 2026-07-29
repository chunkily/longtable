---
title: Fog of war hide/reset/reveal-all controls
created: 2026-07-29
tags: [scenes, gameplay]
---

Fog of war is currently reveal-only, and only cell-by-cell. `internal/store/fog.go` only has
`RevealCells` (an idempotent insert) and `ListFogCells` — there's no way to hide a cell back once
it's been revealed, no way to reset a scene's fog entirely, and no bulk reveal-all either. Scoped
to manual controls only for now — automatic vision-based fog (tokens auto-revealing based on a
sight radius) would need a walls/obstructions data model that doesn't exist at all today, and is
deliberately out of scope for this pass.

## Related user stories

- [gm-hide-fog-cell](../../user-stories/gm-hide-fog-cell.md)
- [gm-reset-scene-fog](../../user-stories/gm-reset-scene-fog.md)
- [gm-reveal-entire-scene-fog](../../user-stories/gm-reveal-entire-scene-fog.md)
