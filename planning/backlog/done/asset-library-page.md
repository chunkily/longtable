---
title: A page for the asset library, with names, attribution and grid alignment
created: 2026-08-03
tags: [assets, ui, maps]
---

Give the library its own page at `/r/{slug}/assets`, and make it the place assets are prepared
before they're used at the table: uploaded, named, credited, and — for maps — aligned to the grid.
The in-room picker becomes pick-only.

The [library](../done/asset-library.md) shipped as a grid embedded in the scene and token dialogs,
which is the wrong shape for two of the things it now has to do. Attribution is typed *before*
choosing a file and applies to "the next upload", which reads as a trap. There's nowhere to put a
grid alignment step at all — the dialog it lives in is already a form about something else. And
the search item warns against bolting a filter onto a grid that wasn't built for one.

Subsumes [asset-library-search](asset-library-search.md) and
[map-asset-grid-offset-padding](map-asset-grid-offset-padding.md); both should move to
`done/` with this rather than surviving it.

## Decided up front

Settled from mockups before any code, so a later reader doesn't re-litigate them:

- **Library first, upload folds in.** The page is the search field and the library grid; picking
  files stacks an editor card per file above the grid. Not a two-pane workbench with a staging
  queue — one column survives being narrow, and the search-plus-grid is then the same component
  the picker embeds, which is what the search item asked for.
- **Nothing reaches the server until "Add to library".** Staging is entirely client-side, off an
  object URL. So there is no half-finished asset row, no draft state to clean up, and the upload
  carries name, attribution and grid offset in one request — which is what lets the offset be
  baked at re-encode time as [ADR-0005](../../decisions/0005-webp-reencoding-library.md) and the
  grid story require.
- **The picker stops uploading.** Scene and token dialogs choose from the library and link here
  instead. Two upload paths would mean one that quietly produces unnamed, unaligned assets, and
  the names are what search runs on.
- **The grid step records the square size as well as baking the offset.** Baking alone leaves
  `scene.gridSize` typed by hand in the scene dialog, where guessing it wrong undoes the
  alignment. Measured size is stored so scene creation can default to it.
- **Name and attribution are editable afterwards; the grid isn't.** Names default from filenames,
  so a first pass is full of `dungeon_final_v2` and search is only as good as they are. The grid
  can't be re-done because changing pixels makes a different asset by definition.

## Data model

Both new columns go on `room_asset`, not `asset`:

- `name` — per-room for the same reason attribution is. Defaults to the filename minus its
  extension, client-side, so it's a real editable value and not a fallback computed at render.
- `grid_size` — nullable; null means "not a map, or never aligned". This is the arguable one,
  since a square size is a measurement of the pixels and `asset` rows are content-addressed, so
  it would be *correct* globally. Per-room anyway: a global column means one room's upload writes
  a value another room reads, and the write rule for that ("fill only when null"? "last one
  wins"?) is a decision with no good answer. Two rooms that both align the same map each measure
  it, which is work they were doing regardless.

`grid_offset_x`/`grid_offset_y` get no column anywhere — they're baked into the pixels and then
forgotten, which is the whole point of the grid item. The dead `scene.grid_offset_*` fields stay
dead.

## Work

- [ ] `room_asset.name`, `room_asset.grid_size`, both via `addColumnIfMissing`
- [ ] Padding/cropping in `imageproc` at re-encode, with the `MaxPixels` check applied to the
      padded result rather than the upload
- [ ] Upload accepts `name`, `gridSize`, `gridOffsetX`, `gridOffsetY` alongside `attribution`
- [ ] `PATCH /api/rooms/{slug}/assets/{id}` for name and attribution
- [ ] `/r/{slug}/assets` — search, library grid, per-file editor cards, grid alignment control
- [ ] A shared library component the picker embeds, so search lands in both at once
- [ ] Picker loses its upload button and attribution field, gains a link here
- [ ] Scene dialog defaults its grid size from the picked map's recorded square size

## What shipped

`/r/{slug}/assets` is now the only way art enters a room. Choosing files stages them on the page —
locally, off an object URL, with nothing sent yet — each with a name pre-filled from the filename,
an optional credit, and an optional "Align to grid" step. `Add to library` sends the file and all
of it in one request. The library below is searchable, and every picker in the room embeds the
same component, so search landed in both places at once. The pickers no longer upload; they link
here instead.

What each layer does with the grid figures is the part worth carrying forward:

- **The offset never becomes data.** `imageproc.ReencodeOffset` pads (or, negative, crops) during
  the re-encode every upload already goes through, so the stored WebP is *already* aligned and
  nothing downstream knows an offset existed. `scene.grid_offset_x/y` are still dead and this did
  not revive them. The `MaxPixels` check runs a second time inside `applyOffset`, because the
  header check bounded the upload and padding happens after it — an 8000×8000 map that passed on
  the way in can still be pushed past the cap by a big enough pad.
- **The square size does become data**, on `room_asset`, and the scene dialog defaults to it. That
  pairing is the point: a scene created at the wrong grid size undoes the alignment exactly as
  thoroughly as a wrong offset would, so baking the offset without recording the size would have
  been half a feature.
- **`paddingForOrigin` pads rather than crops**, so a map whose squares start 12px in gains 58px of
  transparency rather than losing a 12px strip of art. It runs the modulo twice because JavaScript
  `%` keeps the sign of its left operand, and a negative pad is silently a crop.

Three things that would cost time to rediscover:

- **`AddAssetToRoom` only overwrites a field when something was supplied** — name, credit and grid
  size each independently. Re-adding a file the room already has is how people discover it was
  already there, and a token upload carries none of the three; without this it would blank them.
  `UpdateRoomAsset` (the PATCH behind Edit) is the opposite and writes both fields as given,
  because there an empty credit means "clear it" rather than "I had nothing to add".
- **The upload handler reads the entry back** rather than echoing what it was sent, so a re-add
  returns the details the library actually holds.
- **The e2e fixtures are 8x8 and the server floors a square at 8px**, so the alignment spec uses an
  8px square offset by 1 — padding to 15x15. A smaller square looks reasonable against a tiny
  fixture and is a 400.

Not done, and deliberately: nothing warns that the pickers' library list can go stale if someone
adds art in another tab mid-session, which is why the link opens in one. Removing an asset from a
library still doesn't exist — see [track-removed-asset-hashes](../open/track-removed-asset-hashes.md).

## Related user stories

- [room-member-browse-asset-library](../../user-stories/room-member-browse-asset-library.md)
- [room-member-upload-library-asset](../../user-stories/room-member-upload-library-asset.md)
- [room-member-search-asset-library](../../user-stories/room-member-search-asset-library.md)
- [room-member-align-map-grid-offset](../../user-stories/room-member-align-map-grid-offset.md)
- [room-member-name-library-asset](../../user-stories/room-member-name-library-asset.md)
