---
title: Let a room take an asset off its own library
created: 2026-08-03
tags: [assets, ui]
story: room-member-remove-library-asset
---

A library only grows. Every false start, every duplicate uploaded before someone noticed it was
already there, every map for a campaign that ended — all of it stays in the grid and in every
picker, forever.

Removing is a `room_asset` row, not a file. The `asset` row and the blob behind it are global and
content-addressed, so deleting either would reach into other rooms that added the same picture.
That's the distinction to hold on to: this is a room clearing its own shelf.

Not to be confused with [host-moderate-assets](../../user-stories/host-moderate-assets.md), which
is the *opposite* operation — a Host erasing a file everywhere, repointing whatever used it, and
blocking re-uploads by content hash (see [track-removed-asset-hashes](../open/track-removed-asset-hashes.md)).
That one is deliberately a CLI, deliberately global, and deliberately hard to undo. This one is
none of those things.

- [ ] `Store.RemoveAssetFromRoom`, deleting the library row only
- [ ] `DELETE /api/rooms/{slug}/assets/{id}`, scoped like the PATCH beside it
- [ ] Remove on the tile, behind a confirm

## What shipped

Every tile on the assets page has Remove next to Edit; the first click swaps it for a confirm that
names what it's about to do, matching how scene deletion asks. `DELETE
/api/rooms/{slug}/assets/{id}` returns 204, and 404 both for an asset that doesn't exist and for
one belonging to another room — the same blind spot the rest of the asset endpoints keep, so a
removal can't be used to probe what other rooms hold.

What it deliberately does *not* do, all of it tested because all of it is the kind of thing a
later change breaks by accident:

- **The file survives.** Another room holding the same picture keeps it, and adding the file here
  again brings it back — as a fresh row, so the old name and credit don't come with it. The row is
  gone, not archived.
- **It doesn't reach onto the table.** A scene or token already using the image goes on using it:
  they hold the asset ID, which is still good, and `GET /api/assets/{id}` still serves the bytes.
  Yanking the art out from under a scene mid-session is not what "remove it from the library"
  asks for, and the alternative — refusing removal while anything uses it — makes tidying up
  impossible in exactly the room that most needs it.
- **Any room member can do it**, like uploading and renaming. The damage is bounded and
  reversible, which is what separates this from deleting a scene.

Nothing is broadcast: the library isn't part of `state.sync`, so someone else's open picker keeps
showing the removed asset until it refetches. That's the same staleness the picker already
documents for *additions*, and it's why the "Add images" link opens in a new tab.
