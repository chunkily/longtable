---
title: Asset library
created: 2026-07-29
tags: [assets, maps, tokens]
---

Change from uploading maps and token images for every single instance to instead pick from an asset library.

- Asset library is scoped per room (e.g. `/r/{slug}/assets`), not global — keeps private rooms' assets private.
- Add an optional attribution and license metadata field for each asset (perhaps just a single free text field instead of two separate ones).
- Needs search/filter from the start, not as a later addition: a room accumulates dozens of
  uploads fast, and a plain unfiltered grid stops being usable well before that. Design the
  library listing (and the picker embedded in scene/token creation) around a search field
  narrowing the same list, rather than bolting filtering onto a grid built without it.

## Related user stories

- [room-member-browse-asset-library](../../user-stories/room-member-browse-asset-library.md)
- [room-member-search-asset-library](../../user-stories/room-member-search-asset-library.md)
- [room-member-upload-library-asset](../../user-stories/room-member-upload-library-asset.md)
- [gm-pick-map-from-library](../../user-stories/gm-pick-map-from-library.md)
- [gm-pick-token-image-from-library](../../user-stories/gm-pick-token-image-from-library.md)
