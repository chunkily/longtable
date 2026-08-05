---
title: Room Member removes an asset from the room's library
created: 2026-08-03
status: done
---

As a Room Member
I want to take a picture off my room's library
So that the shelf holds what we're actually using, instead of every false start and duplicate
anyone has ever uploaded

## Acceptance criteria

- [ ] An asset can be removed from the library it's in, from the assets page
- [ ] Removing takes more than one click, so a stray click can't empty a library
- [ ] Removing is scoped to this room: another room holding the same picture keeps it
- [ ] A scene or token already using the picture goes on showing it
- [ ] Adding the same file again puts it back in the library
- [ ] The removal survives a reload, and the picture is gone from the room's pickers too
