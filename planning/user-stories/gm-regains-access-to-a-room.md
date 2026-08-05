---
title: GM regains access to a room they've lost
created: 2026-08-05
status: done
---

As a GM
I want to know what to do when I've lost my room's link or my GM password
So that a cleared browser or a forgotten password doesn't cost me a campaign

## Acceptance criteria

- [ ] There is a documented answer for a lost link, and a documented answer for a lost GM password
- [ ] Both are reachable from the README, so a GM who doesn't run the server can still find them
- [ ] Identifying the room requires only what a GM would actually remember — its name, or roughly when they made it

## What already does this

Nothing in the product: the answer is "ask your Host", and this story is satisfied by that being
written down rather than by code. It's recorded as a story anyway because it is exactly the kind
of requirement that gets lost — the mechanism already existed in
[host-restores-room-access](host-restores-room-access.md) and nobody outside the source tree could
have known it did.

Written up in `docs/hosting.md`, with a pointer from the README for the GM who is not the person
running the server, which is the normal case.

Deliberately not a web feature. A "recover my room" page reachable by anyone who can load the
server would hand out room links to whoever asked, which is the thing
[gm-set-room-visibility](gm-set-room-visibility.md) was dropped for being. Routing it through a
human who already has the database is the point, not a limitation.
