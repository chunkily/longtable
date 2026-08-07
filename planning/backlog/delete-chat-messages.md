---
title: Delete chat messages
created: 2026-07-29
status: done
tags: [chat]
---

GM can delete chat messages. Users can also delete their own messages.

**Deletion is two-stage with per-viewer redaction, decided 2026-08-07.** The first `chat.delete`
on a message soft-deletes it, but the row's content is never cleared — only the author and
whoever just deleted it keep seeing the original text (struck through client-side); everyone else
gets a "this message has been deleted" placeholder. A second `chat.delete` on an already-deleted
message purges it outright for everyone. This mirrors the hidden-token precedent in
`token.update`: `chat.deleted` goes out per-client via `broadcastPerClient`, the same technique
that withholds a hidden token from anyone but its owner and the GM.

Authorization for *both* stages is the same author-or-GM check against the message's original
author — a GM may delete or purge any message, a Player only one they sent. That's a different
question from *who still sees the content* after the first stage, which is author-or-deleter, not
author-or-GM: a GM moderating a Player's message is a deleter who isn't the author, and should
still see what they just removed, but a GM deleting isn't automatically added to every other
message's visibility. The row has to survive the first stage either way — a purge with nothing
left to check authorship against would have to fall back to GM-only, silently taking the second
click away from the Player who made the first.

There's no deletion capability at all today — no `DeleteMessage` in `internal/store/message.go`,
no `"chat.delete"`-style case in the WS handler switch (only `"chat.send"` exists,
`internal/ws/hub.go:207`), and no soft-delete column on the `message` table. Needs:

- [ ] A `deleted_at` and a `deleted_by_participant_id` column on `message` (both nullable, via
      `addColumnIfMissing`)
- [ ] A store method to soft-delete (stamp both columns, leave content alone) and one to hard-delete
- [ ] A `chat.delete` WS command: soft-deletes if the message isn't yet deleted, purges if it is —
      same author-or-GM check either way (via `Participant.Role`/`ID`,
      `internal/store/participant.go:14-27`)
- [ ] A viewer-aware payload builder: full content for the author or the deleter, blanked fields
      (with a `deleted` flag) for everyone else — so a client never has to decide for itself
      whether to redact
- [ ] `state.sync`'s message history applies the same per-viewer split for a client connecting
      after the fact, and never sends a purged message at all

## Related user stories

- [gm-delete-any-chat-message](../user-stories/gm-delete-any-chat-message.md)
- [player-delete-own-chat-message](../user-stories/player-delete-own-chat-message.md)

## What shipped

All of the above. `message` gained nullable `deleted_at` and `deleted_by_participant_id`
(`internal/store/store.go`, `internal/store/message.go`), plus `GetMessage`, `SoftDeleteMessage`
(stamps both columns, content untouched) and `DeleteMessage` (hard delete). `handleChatDelete`
(`internal/ws/hub.go`) does both stages behind one command: author-or-GM check first, then either
a soft-delete broadcasting `chat.deleted` per-client, or a purge broadcasting `chat.purged` to
everyone alike (a purged message has no content left to redact, so it needs no per-viewer split).

`messagePayload` (`internal/ws/state.go`) takes a `viewerParticipantID` and blanks body/roll
fields unless the viewer is the author or the recorded deleter (`messageVisibleTo`); `state.sync`
passes the connecting client's own id so a reconnect gets the same split live traffic would have.
`RoomClient` no longer redacts anything itself — it just takes whatever `chat.deleted` payload the
server sent for that client and swaps it in. The chat log in `web/src/routes/r/[slug]/+page.svelte`
tells "redacted" from "privileged view of a deleted message" by whether `body` (or `rollExpression`)
came through empty: privileged renders the normal content struck through and dimmed; redacted
renders the generic placeholder. The delete button itself is still offered to whoever may *act* on
the message (GM: any; everyone else: their own) — that's the author-or-GM authorization question,
separate from the author-or-deleter visibility question above — and its label changes once the
message is already deleted, so the two clicks read as two different actions.

Covered by Go hub tests (`internal/ws/chat_delete_test.go`) for: author sees content after their
own delete, a bystander sees only the placeholder, a GM deleting someone else's message leaves
both the GM and the original author privileged while a bystander still isn't, and a client
connecting fresh via `state.sync` gets the same split a live client would have. Vitest cases in
`room.svelte.test.ts` cover both the privileged and redacted payload shapes reaching `RoomClient`
unmodified. A manual browser pass confirmed the strikethrough CSS (`line-through`, dimmed opacity)
actually renders and that a purge still removes the row for good. No Playwright spec — `chat.send`
itself has never had one either, and this doesn't touch canvas pixels or a disconnect, the two
things that usually earn a feature one here.

One thing worth not rediscovering: the row keeping its real content through the first delete stage
is the whole design, not an intermediate step on the way to clearing it. An earlier pass here
cleared `body`/`roll_expression`/`roll_result`/`roll_breakdown` on soft-delete, which is the right
call if *everyone* should see the same placeholder — it stopped being right the moment "the
deleter and the author still see it struck through" became a requirement, since redaction then has
to happen per viewer at read time, and a row that already forgot the content has nothing left to
show the two people who are supposed to still see it.
