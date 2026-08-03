---
title: Split the asset library into Tokens and Maps, and show token art square
created: 2026-08-03
tags: [assets, ui]
story: room-member-separate-token-and-map-art
---

Feedback on [asset-library-page](asset-library-page.md) as shipped: one flat grid mixing token art
with battle maps is hard to read, and the tile crops a token to a 16px-tall letterbox — which is
the one shape a token never is, so you can't see what you're picking.

Two tabs, Tokens and Maps. Token tiles square and uncropped; maps keep the wide crop, since a map
is never square and never legible at thumbnail size anyway.

The kind has to be real data, not a guess. Today the only signal is `grid_size`, set when someone
aligned a map — but alignment is optional, so a map added in a hurry is indistinguishable from
token art, and that guess would be wrong exactly when someone is looking for the map they just
added.

- [ ] `room_asset.kind`, defaulting to `token`, with the migration sorting existing rows
- [ ] Upload and `PATCH` carry it; the store keeps its "empty means not supplied" rule
- [ ] Tabs in the shared library component, so the assets page and every picker get them at once
- [ ] The assets page asks for the kind while preparing a file, and can correct it afterwards
- [ ] Pickers open on the kind they want without hiding the other one

## What shipped

`room_asset.kind` is `'token'` or `'map'`. The library grid — shared by the assets page and all
four pickers — is tabbed, counts each tab, and draws token tiles `aspect-square object-contain` so
the whole picture shows. The scene and replace-map pickers open on Maps, the token pickers on
Tokens, and both tabs stay reachable from either.

**The whole assets page is tabbed, not just the grid**, and that was a correction made during
review. The first cut asked for the kind on each staged file, with a Token/Map toggle in the
card's action row — which puts the question *after* choosing the file, which is after the
interesting part is over, which is when it gets skipped. Now the tab is picked first and decides
what an upload will be: the card retitles itself ("Add maps" / "Choose maps"), and the staged card
states the answer rather than asking for it. `AssetKindTabs` exists so the same switch can govern
a grid in the pickers and a whole page here without becoming two controls that look alike and
mean different things.

**The image's shape questions the choice but never overrides it.** `guessAssetKind` in
`web/src/lib/asset-kind.ts` reads a staged file's dimensions and, when they disagree with the open
tab, the card says so and offers one click to move it. The rule is *squareness first*: token art
is square by convention, so anything far off square, or square but over 1200px, reads as a map.
A plain pixel threshold — the obvious first idea, and what was originally suggested — doesn't
survive contact with real art, because token art is routinely 256–1024px and any threshold low
enough to catch maps calls almost every token a map too. The guess is wrong often enough (square
maps, banner-shaped art) that it prompts rather than decides, and it shows the dimensions it's
arguing from so the reader can judge it.

The decisions worth not rediscovering:

- **The kind is per-room, on `room_asset`, for the same reason the name is** — and here it isn't
  even arguable: the same dragon is one group's boss portrait and another's mural.
- **A guess, once corrected, must not be re-guessed.** The migration classifies old rows by
  `grid_size IS NOT NULL`, and it runs *only on the boot that adds the column* — which is why
  `addColumnIfMissing` now reports whether it actually added anything. Tying the backfill to
  `migrate()` instead would silently undo every correction on the next restart.
- **An absent kind means "not supplied", never "token".** `AddAssetToRoom` binds it three times
  rather than reading `excluded.kind`, because by the time `ON CONFLICT` sees the row the column
  default has already been applied and the sentinel is gone. Without that, an old client re-adding
  a file it knew nothing about would file the room's map under Tokens. `UpdateRoomAsset` keeps the
  same rule for the kind alone — an empty credit means "clear it", but there's no third kind for
  an empty kind to mean.
- **Pickers seed the tab, they don't lock it.** A map filed as token art (by a hurried upload, or
  by the migration's guess) has to stay reachable from the scene dialog, or the split turns a
  cosmetic mistake into lost art. Same reason the "nothing matches" state counts the hits in the
  other tab and offers to switch.
- **The tab strip is always both tabs**, even at zero. A tab that appears once it has something in
  it can't tell you where the thing you're missing should have gone.

- **The picker's link to the assets page carries the open tab** as `?kind=`, and the page seeds
  its tab from it. Following "Add maps" out of the scene dialog and landing on Tokens would file
  the next upload as token art, which is precisely what the tabs exist to prevent. The page reads
  the parameter once rather than deriving the tab from the URL: after that the tab belongs to
  whoever is looking at the page, and a derived one would snap back on every click. The query
  string can therefore go stale, which costs nothing short of a reload and is cheaper than a
  history entry per tab click.
- **The library grid's `kind` is bound even where its tabs are hidden.** The assets page passes
  `showTabs={false}` and renders the switch itself, but the grid's empty states still offer to
  look in the other half — and an unbound prop had those switching the grid while the tab strip
  above went on claiming otherwise. The e2e spec caught it; it is not obvious by reading.

Known and accepted: nothing validates the kind against the image beyond the prompt. A 4000px
battle map can still be filed as a token if someone clicks past the warning — it'll look odd
squeezed into a square tile, which is feedback enough.

## Related user stories

- [room-member-separate-token-and-map-art](../../user-stories/room-member-separate-token-and-map-art.md)
- [room-member-browse-asset-library](../../user-stories/room-member-browse-asset-library.md)
