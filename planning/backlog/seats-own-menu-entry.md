---
title: Seats get their own menu entry, and Players can read them
created: 2026-08-16
status: done
tags: [manage-room, seats, identity]
---

`Manage room` had grown into two unlike halves: the roster — who is at this table, what colour
they are, who is here right now — and the settings that govern the room, which since
[gm-reset-room-password](gm-reset-room-password.md) and [delete-room](delete-room.md) means the
movement lock, the GM password and deleting the room.
The dialog was GM-only because half of it had to be, which left the other half unreadable by the
people it was about. A Player had no way to see the table's seats at all after joining, and the
one control on that list that is genuinely theirs — their own colour — was somewhere else
entirely, on a popover hanging off the rail.

## What shipped

`Seats` is its own entry in the room menu, above Assets, and **everyone gets it**. The dialog holds
the roster; a GM also gets the `Add a seat` form and a bin on every row but their own. A Player
gets the list and nothing that changes it, which is the same line the server already drew —
`createSeat` and `deleteSeat` are both `requireGM`, and neither needed touching for this.

The colour picker moved into that dialog too, sitting open at the foot of the list — your own seat
is marked with a `you` badge, and the seats above the palette are the "who else is wearing what"
it is chosen against. `Manage room` keeps the movement lock, the password and `Delete room`, and
its description changed to say what is actually left in it — it used to open by explaining what a
seat is, which was the only thing in there this took away.

Worth knowing next time this area moves:

- **Nothing about permissions changed on the server, and that is the point.** The split follows
  [ADR-0007](../decisions/0007-the-table-is-trusted.md)'s existing line rather than drawing a new
  one: reading who is at the table is not a role boundary, and `participant.setColor` takes its
  seat from the connection, so a Player picking a colour in a GM-shaped dialog needs no check
  behind it. `isGM` in `seats-dialog.svelte` decides what renders, not what is allowed.
- **A seat's dot reads its colour from the roster, not from the list it is sitting in.** The list
  comes from `GET /seats` when the dialog opens; `participant.setColor` is deliberately not
  optimistic, so your own dot would have sat on the old colour until something refetched. It falls
  back to the fetched value for a seat the roster hasn't got — a chair a GM set out that nobody has
  taken, which arrives over REST and never appears in `state.sync`.
- **The rail's swatch is now a door, not a control.** It kept its `Your colour` label and its place
  on the `playing as` line, because that is where somebody looking at their own name goes; it opens
  the dialog.
- **A bits-ui popover opened inside a dialog is broken, and it fails silently.** The picker was
  first built the way it worked in the rail — a popover hanging off your own row — and it came out
  `position: static` with `opacity: 0`: unpositioned, invisible, and behind the dialog's own
  overlay, which then ate every click aimed at it. **It stays in the accessibility tree the whole
  time**, so `getByRole('radio', …).click()` found it and the spec passed against a palette no
  person could have used. What caught it was driving the built app by hand and hit-testing
  `document.elementFromPoint` at the swatch — nothing in the build says a word. The fix was to drop
  the nested layer and render the palette inline, which is why it sits open rather than behind a
  toggle. Anything that wants to pop up *inside a dialog* has this waiting; popovers over the map
  are unaffected and stay on the primitive.
- **Both specs now click the palette rather than asserting it exists**, which is the only version
  of that test that would have failed on the bug above.
- The new dialog is laid out as header plus a scrolling body
  (`flex max-h-[calc(100dvh-2rem)] flex-col`), which is the shape `manage-room-dialog.svelte` and
  `token-detail-dialog.svelte` had already arrived at for the same reason: `Dialog.Content` is
  `fixed` and centred by transform, so a dialog taller than the window loses its ends off the
  edges rather than scrolling. A full roster plus the add form plus the palette gets there easily.
  The roster's own `max-h-72` scroller went with it — two nested scrollers in one dialog is one
  too many.

## Related user stories

- [room-member-identity-color](../user-stories/room-member-identity-color.md) — unchanged by this;
  its criteria never named where the picker lives
- [room-member-takes-their-seat](../user-stories/room-member-takes-their-seat.md)
