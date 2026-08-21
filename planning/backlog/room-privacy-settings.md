---
title: Room join password
created: 2026-07-29
status: done
tags: [rooms]
story: gm-set-room-join-password
---

Let a GM set an optional join password on their room, independent of the GM password. Unset by
default, so a link alone is enough to get in.

**This item used to be "visibility + join password".** The visibility half was dropped on
2026-08-05 — see [gm-set-room-visibility](../user-stories/gm-set-room-visibility.md) for why, and
[home-page-lists-your-rooms](home-page-lists-your-rooms.md) for what took its place. The file
keeps its old slug so the links into it don't rot.

Dropping that half makes this half more important rather than less. A room link is now the only
way in, and links get forwarded, pasted into the wrong group chat and read over someone's
shoulder. This is the one control that survives a link going where it shouldn't, and until it
exists a leaked slug is unrecoverable short of asking a Host to delete the room.

**Where it surfaces is already decided.** Under `Manage room`, the third entry in the side panel's
menu — see [full-bleed-map-layout](full-bleed-map-layout.md). The container is built and already
holds seats and the owner-only movement toggle from
[token-move-ownership-lock](token-move-ownership-lock.md), so this is a section to add rather than
a dialog to invent. That item is also the worked example for a room *setting*: a column with a
safe default, a GM-only command, and `room.updated` carrying the whole room so the next setting
needs no new event — which is this one.

Worth settling when it's picked up: whether an existing Room Member is kicked out or kept when a
password is added or changed. Keeping them is friendlier and is probably right — the password
gates joining, not being in the room — but it means adding one doesn't evict whoever the GM was
trying to evict, so it should be said out loud rather than discovered.

**Its home now exists.** [full-bleed-map-layout](full-bleed-map-layout.md) shipped a
`Manage room` dialog (`web/src/lib/components/manage-room-dialog.svelte`), opened from the room
menu and GM-only, holding nothing yet and saying so. This is one of the settings it is waiting
for: add it there rather than inventing a second place for room settings to live, and delete the
"nothing to configure yet" paragraph once something is.

## Related user stories

- [gm-set-room-join-password](../user-stories/gm-set-room-join-password.md)

## What shipped

A nullable-in-spirit `room.join_password_hash` (empty string means unset, migrated in for existing
databases the same way `owner_only_movement` should have been — see `addedColumns` in
`internal/store/store.go`), hashed like the GM password rather than compared in plain text, since
this is the one control meant to survive the link itself leaking. `Store.SetJoinPassword` sets,
changes or clears it; empty is accepted rather than run through `rejectShortPassword`, since unset
is this setting's own valid state and not a typo to reject.

Set from `Manage room` (`PUT /api/rooms/{slug}/join-password`, GM-only) in a new section between
the GM password and Delete room, and from the create-room form too (`createRoomRequest.joinPassword`,
validated and applied after the room itself exists, so a room never ends up half configured) — the
same `Password protected?` toggle in both places. `No` doubles as removing a password that's set,
since there's nothing to type on the way to turning it off, and `Yes` reveals the field to set or
change one. No confirm box, unlike the GM password: getting this one wrong locks nobody out of
anything, so there's nothing a second box would be guarding against.

**The password lives on a pre-join step of its own, not a field on the seats screen.** "Which
chair" and "what's the password" are two different questions, and the join screen already asks
one at a time everywhere else. It gates both ways of becoming a Player at `POST /api/rooms/{slug}/join` —
a fresh seat and claiming one already sat in — checked once, before either branch, and skipped
entirely for a resuming `sessionToken`: the setting gates joining, not being in the room, so a GM
adding one mid-session doesn't evict anyone already at the table. `GET /api/rooms/{slug}/seats`
grew a `joinPasswordRequired` boolean (never the password) so the pre-join screen knows to route
through a password step at all — `role` → `password` → `seats`, skipping straight to `seats` when
none is required — rather than blending "which chair" and "what's the password" into one screen.

**That step verifies as you type it, rather than waiting for the seat and the name to be filled in
too.** `POST /api/rooms/{slug}/join-password/check` answers "is this right?" without joining
anything — unauthenticated like `listSeats`, since it has to answer before there's a session —
sharing its check (`acceptsJoinPassword`) with `joinRoom` so the two can't disagree. Without it, a
wrong guess would only surface after a Player had also picked a seat, a colour and typed a name,
all of which `joinRoom` would then discard on the refusal. The real join still verifies for
itself rather than trusting the earlier check, since a GM could change the password in the gap
between the two calls.

Unlike the GM password, whether one is *set* is visible to the room — the seat list already has to
say so unauthenticated, so it isn't a new exposure — and `roomPayload`'s `joinPasswordSet` carries
it over `state.sync`/`room.updated` the same way `ownerOnlyMovement` does. The wrinkle: this
setting is changed over REST rather than a WS command (it's a credential, like the GM password, not
table state), so there's no command handler already broadcasting the result. `Hub.BroadcastRoomUpdated`
is the new seam — the REST handler's way of sending the same event a command handler would, so a
GM's own second tab watching Manage room updates live rather than on reload.

Two pre-existing doc lines said "the room password" meaning the GM password, which stopped being
unambiguous the moment a second, genuinely different room-scoped password existed — reworded to
"the GM password" in `CLAUDE.md` rather than left to read as this one.
