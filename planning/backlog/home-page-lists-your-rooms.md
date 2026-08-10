---
title: The home page lists your rooms, not the server's
created: 2026-08-05
status: done
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

## What shipped

The home page lists the rooms this browser has sessions for, most recently opened first, each row
carrying the room's name, the name you go by there and whether you're its GM. Below it, a box that
takes an invite; below that, the create form as before. `GET /api/rooms` is gone.

**Superseded in part, 2026-08-10.** The list and its ordering are unchanged, but the two things
under it are not: they're behind large buttons on a welcome screen now, and the empty state that
this item added is gone rather than reworded. See
[home-page-welcome-screen](home-page-welcome-screen.md). `parseInvite` is `parseRoomCode` in
`web/src/lib/room-code.ts`.

**And it is no longer lenient**, which reverses the paragraph below: the join box became one large
six-character field, so the parser was narrowed to match it. A URL, a path and a trailing slash are
all refused now. The reasoning below for the *alphabet* still holds and is the durable half —
`0`/`o`/`1`/`l`/`i` are still excluded, still mirrored from `internal/store/slug.go`, and still for
the same reason.

**The design question in the item above answered itself: the session already holds the room name.**
`sessionResponse` has carried `roomName` since rooms were first built, so neither of the two
options — cache it at join time, or add an endpoint to resolve slugs — was needed. Worth the two
minutes it takes to look: the plan assumed a gap that wasn't there.

The one thing genuinely missing was ordering, so `saveSession` now stamps `lastOpenedAt` and the
room page calls `touchSession` on mount. That's deliberately *not* folded into `loadSession`: the
assets page loads a session too, and reading one isn't the same as sitting down at the table.

**`GET /api/rooms` was deleted rather than filtered.** Leaving it up and hiding the UI would have
kept the leak while making it invisible, which is worse than the honest version — the next person
reads the page and assumes rooms are private. `store.ListRooms` stays, because
`longtable room list` is the Host's enumeration path and needs the database file, which is the
right bar for it.

The Go test asserts on the *room name* not appearing in the response rather than on a status code.
Whether an unregistered method falls through to the SPA route, 404s or 405s is an implementation
detail; that nothing leaks is the property. It also names the room something distinctive, because
the existing `createTestRoom` calls its room "Room" — common enough to appear in an unrelated body
and pass for the wrong reason.

`parseInvite` (`web/src/lib/invite.ts`) is lenient about shape and strict about the slug: a full
URL, a path or six bare characters all work, case folded because phones capitalise the first
letter unprompted, but the last segment has to match the slug alphabet. That alphabet is mirrored
from `internal/store/slug.go`, which drops `0`/`o`/`1`/`l`/`i` precisely so a code read aloud isn't
ambiguous — the two have to agree, and this is the situation that alphabet exists for.

### Found in passing

The room-creation e2e flake was **mis-diagnosed** when it was fixed; the trigger was right and the
mechanism was wrong. Corrected in place in [e2e-flakes](e2e-flakes.md) — it's a hydration race that
loses form values, not a click landing where a button used to be, and it can return on any page
that grows enough to hydrate slowly.
