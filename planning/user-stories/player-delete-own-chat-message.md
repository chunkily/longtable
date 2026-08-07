---
title: Player deletes their own chat message
created: 2026-07-29
status: done
---

As a Player
I want to delete a chat message I sent
So that I can remove something I posted by mistake

## Acceptance criteria

- [ ] A Player can delete or purge a chat message only if they sent it
- [ ] Attempting to delete or purge someone else's message has no effect
- [ ] Deleting their own message for the first time, in real time: they still see the original
      content struck through (they're both the author and the deleter); everyone else in the room
      sees a "this message has been deleted" placeholder instead
- [ ] Deleting an already-deleted message of theirs removes it entirely for everyone in the room
      in real time
- [ ] Both stages happen server-side — the redaction is decided per viewer on the server, not
      hidden or shown by client-side logic
