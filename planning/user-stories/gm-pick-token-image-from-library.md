---
title: GM picks a token image from the asset library
created: 2026-07-29
status: done
---

As a GM
I want to pick a token image from my room's asset library when creating a token, instead of only uploading a new file
So that I can reuse art already used in this room (e.g. common monsters, recurring NPCs) without re-uploading it every time

## Acceptance criteria

- [ ] Token creation shows the room's token library to pick from
- [ ] Selecting an existing asset uses it for the token immediately, with no upload step
- [ ] The image is optional — a token can be created with no art, as a plain marker
- [ ] Adding new art happens on the assets page, not inline in the dialog; the picker links there
      (opened in a new tab, pre-filtered to Tokens) rather than embedding an upload control

Settled 2026-08-05, same reasoning as
[gm-pick-map-from-library](gm-pick-map-from-library.md): the shared `AssetPicker` component
dropped inline upload in favor of linking out to the assets page, since a name/credit/kind
couldn't be captured from inside a dialog about something else. Both dialogs use the same
component, so this criterion set matches it exactly.
