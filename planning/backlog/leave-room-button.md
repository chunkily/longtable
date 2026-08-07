---
title: Leave room button
created: 2026-07-29
status: done
tags: [ui]
story: room-member-leave-room
---

Add a button to leave the room.

**It now has a home.** [full-bleed-map-layout](full-bleed-map-layout.md) puts it in the menu behind
the third icon at the foot of the side panel, alongside Scenes, Assets and Manage room — decided in
the 2026-08-04 design session. That answers the only open question this item had, which was where
a button nobody wants to hit by accident should live.

**Half of it already exists.** [home-page-lists-your-rooms](home-page-lists-your-rooms.md) shipped
a "Forget" on each row of the home page, which calls `clearSession(slug)` — the whole of what
leaving does on the client, since a session in `localStorage` is the entire record of being in a
room. So this item is now the *in-room* entry point for the same action, plus whatever it should
do beyond forgetting.

That "beyond" is the part left to decide, and it's worth deciding rather than inheriting: leaving
could also disconnect the socket and drop the participant from the roster, which Forget
deliberately doesn't do. Forget is a browser tidying its own list; leaving might reasonably mean
telling the room. Note the roster is "everyone who has ever joined", so removing someone from it
is not currently a thing the protocol can express.

## What shipped

`Leave room` is the last entry in the room menu (`room-menu.svelte`), where
[full-bleed-map-layout](full-bleed-map-layout.md) said it would go. It arms in place rather than
firing on one click — the same two-step the scene list uses for deleting a scene — because
rejoining afterwards mints a **new** participant rather than picking the old one back up, so
anything the old one owned stays owned by someone who is no longer you.

The "beyond forgetting" question resolved to: disconnect the socket, then `clearSession(slug)` and
go home. Disconnecting first means the others see the presence drop straight away instead of when
the socket eventually times out. It deliberately does **not** tell the room in any stronger sense —
the roster is everyone who has ever joined and the protocol still has no way to express removal
from it, so a "left the room" event would be a new command in service of a list that isn't meant
to shrink. If that changes, this is the caller to revisit.
