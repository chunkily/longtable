---
title: Delete a room
created: 2026-08-04
status: done
tags: [rooms, gm]
story: gm-delete-room
---

Let a GM delete their room. There is no way to do this today from anywhere — not the UI, not the
`room` admin CLI, which only lists rooms and resets passwords.

Came out of the 2026-08-04 layout design session as one of the things `Manage room` should hold;
see [full-bleed-map-layout](full-bleed-map-layout.md) for where it lives.

Things it has to decide, none of them settled:

- What goes with the room — scenes, tokens, fog, drawings, chat and the participant roster all
  hang off it. Blobs are content-addressed and shared between rooms, so the *files* must survive;
  only the room's library rows go.
- Whether it's recoverable. Everything else destructive in this app is undoable; this one can't
  reasonably be, which makes it the first thing that genuinely needs a confirmation dialog rather
  than an undo.
- What the other people in the room see when it happens under them.
- Whether the `room` CLI should grow the same command, since a Host cleaning up a server has the
  same need and no room password.

**Its home now exists.** [full-bleed-map-layout](full-bleed-map-layout.md) shipped a
`Manage room` dialog (`web/src/lib/components/manage-room-dialog.svelte`), opened from the room
menu and GM-only, holding nothing yet and saying so. This is one of the settings it is waiting
for: add it there rather than inventing a second place for room settings to live, and delete the
"nothing to configure yet" paragraph once something is.

## What shipped

`Delete room` at the foot of `Manage room`, armed then fired like the seat bins, behind
`DELETE /api/rooms/{slug}` and `requireGM`. The four things this item said were unsettled, and
what they were settled to:

- **What goes with it**: everything, in one `DELETE FROM room`. Participants, scenes (and through
  them tokens, fog and drawings), initiative entries, chat and the room's library rows all cascade
  from `room(id)`. The `asset` rows and the blobs behind them stay — they are content-addressed
  and shared, so deleting them would empty another room's library. `TestDeleteRoom_TakesItsContentsAndLeavesTheImages`
  pins both halves, including a second room still holding the same picture afterwards.
- **Recoverable**: no, and that is what the confirmation is for. It is the only control in the app
  guarding something with no undo.
- **What the others see**: a `room.deleted` event, then their socket closes. Their browser forgets
  the stored session and goes home with a line saying what happened. Told, rather than left
  wondering.
- **The CLI**: not built. A Host cleaning up a server is a real need and the story doesn't ask for
  it, so it stays open rather than getting a half-considered second entry point.

Three things that bit on the way and are worth knowing:

- **Closing sockets in a loop is slow.** `conn.Close` writes a close frame and waits up to five
  seconds for the peer's reply, so six players meant a thirty-second HTTP request. Each close now
  runs in its own goroutine and nothing waits for them; the hub test went from 10s to 0.06s and
  would catch it coming back.
- **A deleted room's departure timers fire into a room that isn't there.** Every closed connection
  leaves one, and half a minute later each tries to write a `left` line — a foreign key failure
  and an `ERROR` log per participant, which reads like a bug. `announceDeparture` now checks the
  room still exists first. The room is also dropped from the hub's socket map at deletion time,
  which stops the disconnect path broadcasting `measure.ended` into it.
- **The dialog had grown too tall to use.** Adding this section put the button below the bottom of
  the screen on a laptop, with nothing to say so — Playwright reported it as "outside of the
  viewport" before a person hit it. `Manage room` is now header-plus-scrolling-body like
  `token-detail-dialog.svelte`, and the seat list's own scroller went away with it: one scroller,
  not two.
