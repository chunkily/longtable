---
title: Room Member keeps token art and maps apart in the library
created: 2026-08-03
status: done
---

As a Room Member
I want the library to keep token art and maps in separate tabs, and to show token art whole
So that I can find the piece I'm after without reading past a wall of thumbnails, and can see
what a token actually looks like before I pick it

## Acceptance criteria

- [ ] The library has exactly two tabs, Tokens and Maps, and every asset is in one of them
- [ ] Both tabs are always offered, including when one of them is empty
- [ ] A token's preview is square and shows the whole image, uncropped
- [ ] Which kind an upload is filed under is chosen *before* the file is, not inferred from
      whether it was aligned to a grid and not asked for afterwards
- [ ] A staged file whose dimensions disagree with the kind chosen says so, and offers to move
      itself, without ever changing the choice on its own
- [ ] An asset filed under the wrong kind can be moved to the other one without re-uploading it
- [ ] The scene and token pickers open on the kind they're asking for, and can still reach the
      other one
- [ ] A library that predates the split is sorted into the two tabs rather than arriving empty
