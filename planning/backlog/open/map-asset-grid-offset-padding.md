---
title: Grid offset padding on map asset upload
created: 2026-07-29
tags: [assets, scenes]
story: room-member-align-map-grid-offset
---

Apply grid alignment as padding/cropping baked into a map asset's pixels at upload time, during
the mandatory re-encoding step (see [room-member-safe-asset-content](../../user-stories/room-member-safe-asset-content.md)
and [ADR-0001](../../decisions/0001-webp-reencoding-library.md)), rather than as separate
`GridOffsetX`/`GridOffsetY` metadata applied at render time.

Replaces the originally-planned fix for the dead `GridOffsetX`/`GridOffsetY` scene fields — see
[scene-management](scene-management.md).
