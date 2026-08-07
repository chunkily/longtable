---
title: GM deletes any chat message
created: 2026-07-29
status: done
---

As a GM
I want to delete any chat message
So that I can moderate the room's chat, including messages I didn't send

## Acceptance criteria

- [ ] A GM can delete any chat message, regardless of who sent it
- [ ] Deleting a message for the first time, in real time: the GM (as the one who just deleted it)
      and whoever originally sent it still see the original content struck through; everyone else
      in the room sees a "this message has been deleted" placeholder instead
- [ ] Deleting an already-deleted message removes it entirely for everyone in the room in real time
- [ ] Both stages happen server-side — the redaction is decided per viewer on the server, not
      hidden or shown by client-side logic — so the same split survives a reload/reconnect
