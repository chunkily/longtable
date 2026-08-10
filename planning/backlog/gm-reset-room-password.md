---
title: GM resets their room's GM password from Manage room
created: 2026-08-10
status: open
tags: [manage-room, auth]
story: gm-reset-room-password
---

Right now the only way to change a room's GM password is `longtable room reset-password <code>`,
which the Host runs against the database file for a GM who's locked out
([host-restores-room-access](../user-stories/host-restores-room-access.md)). There's no way for a
GM who still has access to change their own password — to rotate it after sharing it more widely
than intended, or just as routine hygiene, without going through the Host.

`store.SetGMPassword(roomID, newPassword)` (`internal/store/room.go:156`) already does the write
and already hashes with `auth.HashPassword` — the CLI path (`cmd/longtable/room.go:74`) calls it
today, and its doc comment already anticipates a second caller ("used by both the gm-login
recovery flow's admin path and the CLI reset-password command"). This item is that second caller.

- [ ] `PUT /api/rooms/{slug}/gm-password` (naming to match `POST .../gm-login`), gated by
      `requireGM` like `createSeat`/`deleteSeat`, calling `store.SetGMPassword` with the body's new
      password
- [ ] No server-side check of the *current* password — see the "Note on the current-password
      question" in the linked story for why that's a deliberate omission, not an oversight
- [ ] Client function in `web/src/lib/api.ts` alongside `addSeat`/`removeSeat`
- [ ] A form in `manage-room-dialog.svelte`: new password + a repeated confirm field, disabled
      submit until they match, matching the dialog's existing `toast.error` pattern for failures
- [ ] Copy for the fields, the button and any success/error toast — run it through
      `longtable-copy` before landing, same as every other string in that dialog
- [ ] Test coverage at the store layer (already covered by `TestSetGMPassword`) and a new
      `requireGM`-gated handler test alongside `seats_test.go`'s pattern (rejects a non-GM, rejects
      no session, accepts the room's GM)

## Related user stories

- [gm-reset-room-password](../user-stories/gm-reset-room-password.md)
