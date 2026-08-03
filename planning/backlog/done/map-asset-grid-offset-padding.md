---
title: Grid offset padding on map asset upload
created: 2026-07-29
tags: [assets, scenes]
story: room-member-align-map-grid-offset
---

Apply grid alignment as padding/cropping baked into a map asset's pixels at upload time, during
the mandatory re-encoding step (see [room-member-safe-asset-content](../../user-stories/room-member-safe-asset-content.md)
and [ADR-0005](../../decisions/0005-webp-reencoding-library.md)), rather than as separate
`GridOffsetX`/`GridOffsetY` metadata applied at render time.

Replaces the originally-planned fix for the dead `GridOffsetX`/`GridOffsetY` scene fields — see
[scene-management](scene-management.md), which shipped without touching them, so they
are still stored, still sent to the client, and still applied nowhere.

## What shipped

Shipped as part of [asset-library-page](asset-library-page.md), which is where it found somewhere
to live: aligning a map needs a picture on screen and room to drag it, and the scene dialog this
would otherwise have gone in is a form about something else.

`imageproc.ReencodeOffset` applies the offset during the re-encode, so the served image is
already aligned and no offset is stored anywhere. The scene's dead `grid_offset_x/y` columns are
still dead — this item deliberately did not revive them, and nothing reads them.

The one thing this item didn't anticipate: aligning also has to record the **square size** it was
aligned at, or the scene gets created at whatever grid size someone types and the alignment is
undone. That lives on `room_asset.grid_size` and pre-fills the scene dialog. Baking the offset
alone would have been half the feature.
