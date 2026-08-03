---
title: Search and filter the asset library
created: 2026-07-31
tags: [assets, ui]
story: room-member-search-asset-library
---

The [asset library](../done/asset-library.md) shipped as a plain unfiltered grid. Add a search
field that narrows it live, matching against filename and attribution text.

This was meant to land *with* the library rather than after it: a room accumulates dozens of
uploads fast, and an unfiltered grid stops being usable well before that. It didn't, so the trap
to avoid now is bolting a filter onto a grid built without one — the search field and the list it
narrows should be the same component, not a wrapper around the existing `<ul>`.

`AssetPicker` (`web/src/lib/components/asset-picker.svelte`) already loads the room's whole
library once on mount and holds it in a `$state` array, so the filter is a `$derived` over that
array — no new endpoint and no server-side query. The scene and token dialogs embed the same
component, so both pick the search up at once.

The picker is currently the only surface the library has; there's no standalone browsing page
yet, so [room-member-browse-asset-library](../../user-stories/room-member-browse-asset-library.md)
is only half met. Whatever shape the search takes should survive being lifted into that page
later.

- [ ] Search field narrowing the library live, over filename and attribution
- [ ] An empty state for "no matches" distinct from the existing "library is empty" hint
- [ ] Clearing the search restores the full list

## What shipped

Shipped as part of [asset-library-page](asset-library-page.md), which is where the reasoning
lives. The warning in this item — don't bolt a filter onto a grid built without one — is what
produced `AssetLibrary`: the search field and the list it narrows are one component, embedded by
both the assets page and every picker, so neither can have search without the other.

One change of substance from what's written above: matching is over the asset's **name**, not its
filename. Names didn't exist when this was written; they do now, they default from the filename,
and they're what the grid displays — searching a field nobody can see would have been the wrong
half of the pair. `filterAssets` requires every word of the query, in any order, so "archer
goblin" finds the goblin archer.

## Related user stories

- [room-member-search-asset-library](../../user-stories/room-member-search-asset-library.md)
