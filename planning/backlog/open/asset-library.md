---
title: Asset library
created: 2026-07-29
tags: [assets, maps, tokens]
---

Change from uploading maps and token images for every single instance to instead pick from an asset library.

- Asset library is scoped per room (e.g. `/r/{slug}/assets`), not global — keeps private rooms' assets private.
- Add an optional attribution and license metadata field for each asset (perhaps just a single free text field instead of two separate ones).

## Related user stories

- [room-member-browse-asset-library](../../user-stories/room-member-browse-asset-library.md)
- [room-member-upload-library-asset](../../user-stories/room-member-upload-library-asset.md)
- [gm-pick-map-from-library](../../user-stories/gm-pick-map-from-library.md)
- [gm-pick-token-image-from-library](../../user-stories/gm-pick-token-image-from-library.md)
