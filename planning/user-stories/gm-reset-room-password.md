---
title: GM resets their room's own GM password
created: 2026-08-10
status: done
---

As a GM
I want to set a new GM password for the room I'm currently signed into
So that I can rotate it — after sharing it too widely, or just as routine hygiene — without asking my Host to do it for me

## Acceptance criteria

- [ ] From Manage room, a GM can set a new GM password while signed into the room
- [ ] Setting it does not require typing the current password — the session token proves the seat,
      the same as every other action in Manage room
- [ ] A second, repeated field catches a typo before it's saved: getting it wrong here isn't
      recoverable from inside the room, unlike a normal password-change form
- [ ] The change takes effect immediately: a future `gm-login` to this room requires the new
      password
- [ ] It does not affect this device's session, or any other device already signed into the GM
      seat — nobody has to log back in because the password changed under them

## Note on the current-password question

Deliberately not required. [ADR-0007](../decisions/0007-the-table-is-trusted.md) draws the line at
role boundaries, not identity boundaries, and every other Manage room action (`Add a seat`,
`Remove`, the movement toggle) already trusts `requireGM`'s session check alone. Re-asking for the
password here would be a one-off exception to that, and it doesn't defend against the case that
actually happens — a GM who lost the password already has no way to type it, and that path runs
through [host-restores-room-access](host-restores-room-access.md) instead.

## Verified

All five hold. The last two are the ones worth naming: `gm-password.spec.ts` changes the password,
reloads to show the session survived, then leaves and comes back to show the old password is
refused and the new one works; `TestSetGMPassword_LeavesEverySessionSignedIn` covers the second
device on the GM seat, which the e2e can't reach on one browser.
