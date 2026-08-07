---
title: Room join password
created: 2026-07-29
status: open
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
menu — see [full-bleed-map-layout](full-bleed-map-layout.md). It shares that container with
[delete-room](delete-room.md) and the owner-only movement toggle from
[token-move-ownership-lock](token-move-ownership-lock.md), so whoever builds the first of the
three builds the container.

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
