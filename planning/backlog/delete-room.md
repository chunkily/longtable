---
title: Delete a room
created: 2026-08-04
status: open
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
