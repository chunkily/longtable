---
title: Visitor browses public rooms
created: 2026-07-29
status: dropped
---

As a Visitor
I want to browse a list of public rooms
So that I can find and join an open game without needing a direct link

## Acceptance criteria

- [ ] Only rooms marked public appear in the list
- [ ] Private rooms never appear in the list, regardless of who's browsing
- [ ] The list indicates whether a room requires a join password

## Why this was dropped

Decided on 2026-08-05. The reasoning is in
[gm-set-room-visibility](gm-set-room-visibility.md), the other half of the same idea.

This one is also the reason the Visitor role existed, and the file name is left spelling a role
that no longer exists as a marker of that. Neither was designed: the role was introduced in
passing while writing an unrelated branding story, and its definition cited browsing the public
room list as the example of what a Visitor does. So the role justified the feature and the feature
justified the role, and nothing else in the product referred to either. Removing one collapsed
both — see the note at the foot of [roles.md](../roles.md).

Finding a game to join is real, but it happens the way it does for every other game people play
together over a link: somebody sends you one. The replacement is
[room-member-sees-their-own-rooms](room-member-sees-their-own-rooms.md), and for a GM who has lost
the link, [host-restores-room-access](host-restores-room-access.md) — which is the Host's job, not
a browsing feature's.

