---
title: GM picks a map from the asset library
created: 2026-07-29
status: done
---

As a GM
I want to pick a map image from my room's asset library when creating a scene, instead of only uploading a new file
So that I can reuse art already used in this room without re-uploading it every time

## Acceptance criteria

- [ ] Scene creation shows the room's map library to pick from
- [ ] Selecting an existing asset uses it for the scene immediately, with no upload step
- [ ] The map is optional — a scene can be created with no map selected
- [ ] Choosing a map adopts its aligned grid size as the scene's default
- [ ] Adding new art happens on the assets page, not inline in the dialog; the picker links there
      (opened in a new tab, pre-filtered to Maps) rather than embedding an upload control

Settled 2026-08-05, replacing an earlier draft of this criterion that expected inline upload
alongside the picker. That was tried and dropped: an upload made from inside a dialog about
something else had nowhere to put a name or a grid alignment, so it silently produced assets
search couldn't find and maps whose squares didn't line up (see the doc comment atop
`asset-picker.svelte`). Routing upload through the assets page — the one place that already asks
for name, credit, kind and grid offset — is the fix, not a regression to build past.
