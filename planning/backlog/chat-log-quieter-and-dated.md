---
title: Hide the delete button until it's wanted, and date the log
created: 2026-08-15
status: done
tags: [chat, ui]
story: room-member-reads-a-quiet-chat-log
---

Two things the chat panel does badly once a session has any length to it.

**A bin on every line.** The delete button sits permanently beside every message the reader may
delete, which for a GM is all of them — a column of destructive icons down a panel whose job is to
be read. It should appear when the pointer is on a message, and on a touch screen when the message
is tapped, since there is no hover there to appear on.

- [ ] The button is invisible until the message is hovered, focused, or tapped
- [ ] It cannot be clicked while invisible — the first tap reveals, the second acts
- [ ] A keyboard can still reach it, which rules out hiding it outright

**No day boundaries.** Every entry carries a time and nothing says which day it belongs to, so a
log spanning two sessions reads as one long evening where `23:58` is followed by `09:12`. Put the
date above the first message of each day.

- [ ] A date sits above the first entry of each day
- [ ] Today and yesterday say so rather than giving a date to work out

## Related user stories

- [room-member-reads-a-quiet-chat-log](../user-stories/room-member-reads-a-quiet-chat-log.md)

## What shipped

Both halves, in the chat panel and `web/src/lib/chat-time.ts`.

The delete button is `opacity-0` **and** `pointer-events-none` until the message is hovered,
focused, or tapped. The pointer-events half is the one that matters: a transparent button that
still accepts clicks means a first tap on a phone can delete a message that was never on screen,
which is worse than the column of bins this replaces. A tap latches `revealedMessageId` and a
second tap on the same message clears it.

Decisions worth not rediscovering:

- **Tailwind v4 compiles `group-hover:` inside `@media (hover: hover)`**, checked in the built CSS
  rather than assumed. So none of the hover rules apply on a touch screen, the tap is the only
  thing that reveals anything there, and sticky hover can't leave two bins open. Replacing the
  variant with a hand-written `:hover` rule would quietly undo that.
- **No key handler beside the click.** A keyboard reaches the button by tabbing to it, which
  `group-focus-within` reveals; a second route would be one more thing to keep in step. The two
  `svelte-ignore` comments say so where they sit.
- **The e2e for the tap parks the pointer off the message between every step.** Clicking moves it
  there, so a check made straight afterwards is answered by hover and proves nothing — which is how
  the first version of that test passed for the wrong reason. Written properly it asserts the thing
  that actually distinguishes the two: a tap latches, hover only lasts as long as the pointer.
- The author of a deleted message keeps seeing their own text struck through rather than the
  bystander's placeholder, so the e2e asserts on the button becoming `Remove message permanently`.
  The first draft asserted the placeholder and failed against documented behaviour.

The date sits above the first entry of each day — `Today`, `Yesterday`, or the date — compared
against the previous message rather than precomputed into groups, so the list stays flat and keyed
by message id. `sameDay` compares on the reader's calendar rather than the ISO strings, since two
messages either side of midnight UTC are the same evening in Sydney, and an unreadable timestamp is
never the same day as anything so it can't swallow a heading.
