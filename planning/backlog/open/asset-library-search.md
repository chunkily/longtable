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

## Related user stories

- [room-member-search-asset-library](../../user-stories/room-member-search-asset-library.md)
