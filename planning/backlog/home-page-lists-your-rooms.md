---
title: The home page lists your rooms, not the server's
created: 2026-08-05
status: open
tags: [rooms, ui]
story: room-member-sees-their-own-rooms
---

`web/src/routes/+page.svelte` lists every room on the server to anyone who loads it. Replace that
with the rooms this browser has joined, plus a box to paste an invite link or code into, plus the
existing create-room form.

The rooms are already known client-side: `web/src/lib/session.ts` writes one
`longtable:session:{slug}` key per room. Reading them back is a prefix scan of `localStorage` —
there's no index today, so this item either adds one or scans, and scanning is fine at this size.
A session holds the display name and role, so a row can say "GM" or "Player" without asking the
server anything.

What each row needs beyond that — the room's *name* — is not in the session, only the slug. Decide
early whether to store the name at join time (cheap, can go stale if a room is renamed) or fetch
the listed slugs from the server on load (always right, needs an endpoint that takes slugs and
returns names, and has to not become a way to enumerate rooms). The second is the safer shape
given the whole point is that rooms aren't enumerable.

- [ ] List rooms from `localStorage`, newest use first, with the role on each row
- [ ] Join by pasted link or bare code
- [ ] Remove a room from the list — same gesture as [leave-room-button](leave-room-button.md), so
      whoever gets there first should do both
- [ ] Empty state that explains how to get into a room
- [ ] `GET /api/rooms` stops returning every room to everyone

Server-side, the listing endpoint that feeds today's page should go or be locked down in the same
change. Leaving it up would keep the leak while hiding it, which is worse than the honest version
because the next person reads the UI and assumes it's private.

Also fixes, incidentally, the thing that made e2e room creation flaky: the create form currently
sits below an unbounded list, which reached 1031 rooms and about 26,000px — see
[e2e-flakes](e2e-flakes.md).

## Related user stories

- [room-member-sees-their-own-rooms](../user-stories/room-member-sees-their-own-rooms.md)
