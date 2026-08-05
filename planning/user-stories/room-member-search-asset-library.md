---
title: Room Member searches the asset library
created: 2026-07-31
status: done
---

As a Room Member
I want to filter the asset library by typing part of a name or its attribution
So that I can find one image quickly once the room has accumulated dozens of them

## Acceptance criteria

- [ ] A search field narrows the visible assets live as I type, matching against filename and
      attribution/license text
- [ ] Search applies everywhere the library is shown — the library browsing page and the
      "choose from library" picker in scene and token creation
- [ ] Clearing the search shows the full library again
- [ ] A query with no matches shows a clear empty state rather than an empty grid
