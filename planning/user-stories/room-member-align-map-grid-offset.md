---
title: Room Member aligns a map's grid offset on upload
created: 2026-07-29
---

As a Room Member
I want to set a horizontal and vertical grid offset when uploading a new map asset
So that the grid lines up with the map art's actual squares, even when the image doesn't start exactly on a grid boundary

## Acceptance criteria

- [ ] When uploading an asset intended as a map, I can specify a horizontal and vertical pixel offset
- [ ] Positive offsets pad the image; negative offsets crop it, shifting where the image's content starts
- [ ] The offset is baked in during the mandatory re-encoding step every upload already goes through (see room-member-safe-asset-content), so the stored/served image is already aligned — no separate offset value needs to be tracked or applied at render time
- [ ] If no offset is given, the image is stored unchanged (zero padding/crop)
