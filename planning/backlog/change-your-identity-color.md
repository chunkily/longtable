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
