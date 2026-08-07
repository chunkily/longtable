---
title: Leave room button
created: 2026-07-29
status: open
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
