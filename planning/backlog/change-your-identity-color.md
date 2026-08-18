---
title: Change your colour after you've picked one
created: 2026-08-15
status: done
tags: [identity]
story: room-member-identity-color
---

[identity-color](identity-color.md) shipped the choice and nothing else: a colour is picked on the
form that makes a seat and then fixed for the life of that seat. Two people who end up the same
colour by accident — which the room deliberately allows — have no way out of it, and neither does
anyone who simply looks at their name in chat and wants a different one.

- [ ] Somebody in a room can change their own colour, without leaving or retaking their seat
- [ ] The change reaches everyone live, and survives a reload
- [ ] It is their _own_ colour and nobody else's, enforced where identity always is — the
      connection, never the payload

Where it lives is the open question. Colour is seat state rather than browser state, so the room
menu's theme control is the wrong neighbour: that section is explicitly the two things that change
this browser rather than the room. The rail's session block already says who you are, which is the
one place on screen that is _about_ your identity in this room.

## Related user stories

- [room-member-identity-color](../user-stories/room-member-identity-color.md)

## What shipped

The swatch beside `playing as` in the rail's session block, opening the same palette on a popover.
Picking sends `participant.setColor` and closes; the change lands for everyone on
`participant.updated` and is stored, so a reload and a late arrival read it too.

Decisions worth not rediscovering:

- **The command carries no participant id.** The seat changed is the one on the connection, which
  is this protocol's rule everywhere and here means the handler needs no permission check at all —
  a Player changing a colour is changing their own by construction. A test sends a `participantId`
  in the payload anyway and asserts the GM's seat is untouched.
- **The echo goes to the sender too**, and it is deliberately not optimistic. Chat names and pings
  resolve colour from the roster at render, so a client that painted its own change early would be
  the only one in the room showing it until the next sync — and there is no preview shape here that
  would blink, which is the thing optimism buys elsewhere.
- **A change recolours what you already said.** That follows from resolving per render rather than
  stamping a colour onto each message, and it is the right way round: the colour says who someone
  is, not what a message was when it landed.
- The picker grew an `onpick` callback for this. A form holds a value until submit; the rail's
  swatch changes something that already exists, so it sends on click.

**Where it lives changed on 2026-08-16** — the paragraph above describes the rail popover, which is
no longer where the palette opens. The picker moved into the `Seats` dialog when seats got their
own menu entry ([seats-own-menu-entry](seats-own-menu-entry.md)), where it sits open at the foot of
the roster. The reasoning above still holds and is why the rail swatch survived: that line is where
somebody wanting a different colour goes looking, so it is still the way in — it opens the dialog
now instead of the palette. What the move buys is the thing the original couldn't do, which is show
every colour at the table while you choose one; the popover in the rail could only show its own.
`onpick` is unchanged and still the reason this doesn't wait for a submit button.

It is not on a popover any more, and that isn't a change of taste: a bits-ui popover opened inside
a dialog comes out unpositioned, transparent and under the dialog's overlay, while staying in the
accessibility tree — see the note in the other item, and read it before putting anything else on a
popover inside a dialog.
