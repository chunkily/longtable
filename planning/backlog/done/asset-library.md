---
title: Asset library
created: 2026-07-29
tags: [assets, maps, tokens]
---

Change from uploading maps and token images for every single instance to instead pick from an asset library.

- Asset library is scoped per room (e.g. `/r/{slug}/assets`), not global — keeps private rooms' assets private.
- Add an optional attribution and license metadata field for each asset (perhaps just a single free text field instead of two separate ones).

## What shipped

Both the scene and token creation dialogs now open on a shared `AssetPicker` — a grid of the
room's library with an "Upload new image" button rather than a bare file input. Picking a
library entry skips the upload step entirely; uploading joins the library on the way past, so
the second time anyone needs the same map or token art it's already there. An optional
attribution/licence free-text field applies to the next upload and is shown on hover over the
library entry and shared with the whole room, not just the uploader.

Data model: `asset` rows stay global and content-addressed, exactly as before — that's what
[safe-asset-reencoding](safe-asset-reencoding.md) built the dedup on. A new `room_asset` join
table (room, asset, attribution, added_at) is what a room's library actually is.
`AddAssetToRoom` is idempotent and keeps the *existing* attribution when a later add supplies
none, rather than clearing it. `GET /api/rooms/{slug}/assets` requires a session for that room —
a private room's library is a list of everything it has, so handing it to anyone holding the
slug would leak the room's contents.

One scoping hole this closed rather than opened: `requireAssetExists` in the hub previously only
checked that an asset *existed* — any room could point a scene or token at any other room's
asset ID once it learned that ID, since asset rows share one global namespace by design. It's
now `requireAssetInRoom`, checked against the room's library. An asset that doesn't exist and one
that belongs to another room now get the identical error, so the failure can't be used to probe
what exists elsewhere — matching the pattern the WS layer already uses for scenes and drawings.

Two things worth knowing if this area gets touched again:

- The picker loads the library once on mount via `GET .../assets`, and keeps it in sync locally
  after an upload (prepending the new/reused asset, deduped by ID) rather than re-fetching. A
  removal or a change made in a different tab won't be reflected until the picker remounts —
  fine for now since nothing else can currently remove or edit an asset.
- The scene dialog's width/height default to the picked map's *real* dimensions, loaded via a
  fresh `Image()` request rather than trusted from the picker's clipped 64px thumbnail — the
  thumbnail is `object-cover`ed and would give the wrong aspect ratio for anything not already
  square.

## Related user stories

- [room-member-browse-asset-library](../../user-stories/room-member-browse-asset-library.md)
- [room-member-upload-library-asset](../../user-stories/room-member-upload-library-asset.md)
- [gm-pick-map-from-library](../../user-stories/gm-pick-map-from-library.md)
- [gm-pick-token-image-from-library](../../user-stories/gm-pick-token-image-from-library.md)
