---
title: GM sets room visibility
created: 2026-07-29
status: dropped
---

As a GM
I want to mark my room as public or private
So that I control whether it's discoverable by people without a direct link

## Acceptance criteria

- [ ] Rooms are private by default when created
- [ ] Private rooms never appear in any room list; they're reachable only via direct link
- [ ] Public rooms appear in the public room list
- [ ] A GM can change a room's visibility at any time after creation

## Why this was dropped

Decided on 2026-08-05, along with
[visitor-browse-public-rooms](visitor-browse-public-rooms.md), which it only existed to serve.

Public-versus-private presumes an audience of strangers who can reach the server and might
discover a room they weren't invited to. Longtable doesn't have one: a Host runs the binary and
everyone else joins over the LAN, so the set of people who can reach it is already the set of
people who were told where it is. [ADR-0001](../decisions/0001-self-hosted-multi-room.md) chose
self-hosting precisely so this wouldn't be a shared service, and this story quietly assumed it
was one.

The privacy that does matter on a LAN isn't strangers, it's the wrong known person — a Player
seeing the room a GM is prepping, or one table seeing another's. A public/private flag is a poor
fit for that, because the real question is "who is this room for", which the invite already
answers.

Two other things settled it. The correct setting for every intended user would have been
"private", which is the shape of a liability rather than a feature: a column, a settings row and a
filter on every list query, to serve a case the deployment model doesn't produce. And the thing it
was meant to make safe was already unsafe — the home page listed every room on the server to
anyone who loaded it, which is also how the e2e suite ended up rendering 1031 rooms on one page.

What replaces it is smaller and already half-built:
[room-member-sees-their-own-rooms](room-member-sees-their-own-rooms.md). The home page lists the
rooms this browser has actually joined, which is a read of the per-room sessions
`web/src/lib/session.ts` already stores, and needs no server-side notion of visibility at all.

[gm-set-room-join-password](gm-set-room-join-password.md) survives and matters more than before:
once a link is the only way in, gating what happens when one gets forwarded is the whole of access
control.
