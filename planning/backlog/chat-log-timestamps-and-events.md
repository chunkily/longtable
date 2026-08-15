---
title: Chat timestamps, and durable lines when someone joins or leaves
created: 2026-08-15
status: done
tags: [chat, presence]
story: room-member-sees-who-came-and-went
---

Two halves of the same complaint: the chat log says nothing about _when_, and nothing about who
turned up or went home.

**Timestamps** need no server work — `message.created_at` has always been stored and
`messagePayload` has always sent `createdAt`. It is a rendering job in the chat panel.

**Join and leave lines** are durable, and go through the message table rather than a table of
their own. `kind` no longer carries a CHECK constraint (see the note above `addMissingColumns`),
so a third kind is a Go-side change now rather than a table rebuild.

- [ ] Each entry shows the time it landed, with the full date and time available on hover
- [ ] A line in the log when someone joins the room, and one when they leave
- [ ] Those lines persist, so a refresh and a late arrival see the same log
- [ ] They read as the room talking, not as a person talking — no avatar, no "said"

Depends on [presence-departure-grace](presence-departure-grace.md): hung off today's undebounced
disconnect, a "left the room" line would land every time a connection blipped, and unlike a
flickering badge it would still be there an hour later. The grace period is what makes a leaving
line mean something.

**The wording is the client's job, not the database's.** Store the event, not the sentence: a row
that reads `joined` lets `longtable-copy` rule on how it appears on screen, and lets that wording
change without a migration or a log full of two different phrasings.

## Related user stories

- [room-member-sees-who-came-and-went](../user-stories/room-member-sees-who-came-and-went.md)

## What shipped

Timestamps beside every entry (`14:32`, full date and seconds in the `title`), and durable
`joined`/`left` lines rendered as the room talking — centred, muted, no bold name, no delete
button. `web/src/lib/chat-time.ts` holds the formatting with unit tests; the panel is
`+page.svelte`'s `chatPanel` snippet.

Decisions worth not rediscovering:

- **A third message kind rather than a second table.** `kind` lost its CHECK constraint the day
  before this (see the note above `addMissingColumns`), so `system` was a Go-side change. It rides
  the existing `chat.posted` event and `state.sync` message history, which is why the client needed
  no new plumbing at all — a `case` that already existed folds it in.
- **The row stores the event, not the sentence.** `body` is `joined` or `left`; the panel writes
  "Alice joined the room". Rewording it is a copy change rather than a migration, and the log can't
  end up carrying two phrasings of the same event.
- **`participantId` is null on a system line**, and that isn't tidiness. A GM removing a seat while
  its owner is inside their grace window deletes the row a foreign key would point at, and SQLite
  then refuses the very line saying they left — found by an `ERROR` in an e2e run that otherwise
  passed, and now covered by `TestSystemMessage_SurvivesTheSeatBeingRemovedMidGrace`. It also keeps
  the line from being "theirs", which is what `chat.delete` checks.
- **Every connection now writes a line**, so presence is background noise in the chat log as well
  as on the badges. `isPresenceNoise` skips both in the Go tests, and `saidByPeople`/`saidInSync`
  strip them from a stored log or a sync payload — two existing chat tests were asserting on counts
  that now include the room's own lines.
- The arrival's own line reaches them: their `state.sync` was built before it existed, so without
  the echo they'd see everyone else's arrivals and never their own.
