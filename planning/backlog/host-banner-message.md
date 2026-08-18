---
title: Host banner shown to everyone on the server
created: 2026-08-09
status: done
tags: [host, ui]
story: host-announces-to-everyone
---

A Host needs a way to tell everyone on their server something — a restart, a new address, an
outage — without knowing which rooms exist or holding any room's GM password. One message, set
when the server starts, shown across the top of every page until each person dismisses it.

## Shape

- [ ] Set outside the web UI, since a Host has no screen of their own there
- [ ] Served to anyone, session or not: it's shown on the home page of a browser that has never
      joined anything
- [ ] Dismissable per browser, and a *new* message comes back for someone who dismissed the old one
- [ ] Doesn't cover the room's map or toolbar

## Related user stories

- [host-announces-to-everyone](../user-stories/host-announces-to-everyone.md)

## What shipped

`longtable serve -banner "…"` → `GET /api/notice` → a fixed ribbon in the root layout. Three small
decisions worth keeping:

**An unset banner answers 200 with an empty string, not 404.** The client asks on every page load
and no banner is the ordinary case; a status the browser logs as an error for the normal state
would have every Host reporting a bug from their console.

**Dismissal is keyed by the message text**, in `localStorage`, not by a flag. A Host who changes
the banner is saying something new, and it has to reach the people who dismissed the last one.

**The room page had to be told the banner's height.** It is `fixed inset-0`, so it paints straight
over anything the layout puts above it — the map ran under the banner and took the toolbar with
it. The height is measured from the element (`bind:offsetHeight`) rather than assumed from a
class, because the one thing that varies is the Host's sentence and a long one wraps. Two traps
came out of that, both invisible until you compare bounding boxes:

- `clientHeight` excludes the border, so the map started exactly one pixel under the banner.
  `offsetHeight` is the one that includes it.
- Unmounting the element leaves the last bound value behind, so dismissing the banner gave the
  space back on screen and kept the gap it had been occupying. `height` is a getter that returns 0
  unless the banner is actually visible.

Not done: changing the message without restarting. It is a flag, so it is fixed for the life of
the process. [host-config-file](host-config-file.md) is where that belongs — **this is one of the
settings that item must absorb**, and a config file it can reload is what would make the message
editable in place.

**Update, 2026-08-18.** `host-config-file` shipped and absorbed it: the banner is now `banner` in
`longtable.toml` and the flag is gone. The second half of this paragraph is still true, though —
the file is read once at startup, so changing the message still means a restart. Live reload was
deliberately left out of that item and is the note at the foot of it.
