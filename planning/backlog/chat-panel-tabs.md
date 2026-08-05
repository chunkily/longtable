---
title: Chat panel with tabs
created: 2026-07-29
status: dropped
tags: [chat, ui]
---

Move chat into a panel on the right under a chat tab. Have a separate tab for the initiative
tracker. (An event log tab was considered and dropped as redundant with chat.)

## What shipped

**Nothing, directly — this was superseded before it was built.** It was marked `done` until
2026-08-05, for want of anywhere better to put it; `dropped` now exists and says the same thing
without claiming a build that never happened.

A design session on 2026-08-04 replaced the whole room page rather than the chat card inside it:
see [full-bleed-map-layout](full-bleed-map-layout.md). The panel there is a full-height
rail rather than a card, and the switch between chat and the initiative tracker is three icons at
its foot rather than a tab strip. Everything this item wanted is in that one, including the two
things its story was careful about — an in-progress chat draft surviving a switch, and the same
behaviour on the mobile sheet as on the desktop panel.

Two decisions made here that survived into it, and are worth not re-litigating: an event-log tab
was considered and dropped as redundant with chat, and the who's-connected badges stay outside the
switched region rather than scrolling away with the chat. The second is recorded as a comment in
`+page.svelte` next to the `whoIsHere` snippet, which is where it'll actually be read.

## Related user stories

- [room-member-chat-panel-tabs](../user-stories/room-member-chat-panel-tabs.md)
