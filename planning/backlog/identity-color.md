---
title: Room Member identity color
created: 2026-08-03
status: open
tags: [identity]
story: room-member-identity-color
---

A preset color, chosen per room, shown next to pings and chat names.

This used to be scoped apart from a device-level display name, on the grounds that colour was
room-level and the name wasn't. That contrast is gone: the device-level name was dropped on
2026-08-05 ([room-member-reusable-display-name](../user-stories/room-member-reusable-display-name.md)),
so colour and name are now both properties of a seat and picked in the same place — the seat
picker. They share a mechanism after all, which is the opposite of what this item used to warn
about.

**Colour belongs on the seat**, decided 2026-08-05 with
[ADR-0008](../decisions/0008-seats-and-sessions.md). That resolves the second question below and
strengthens the first: a colour attached to a durable seat survives a device change alongside the
tokens, which is most of what makes it worth having. It's also what makes this story's criterion
"tied to my participant record in this room, not to my device" actually true — when it was
written, `participant` *was* the device, so that line would have passed review and then failed the
first time someone cleared their browser.

This now depends on [seats-and-sessions](seats-and-sessions.md) and should not be picked up first.

Not yet decided, worth settling before picking this up:

- [ ] Whether this ships at all — still weighing whether it earns the wire surface below
- [x] ~~Where picking happens~~ — settled by the seat model. The seat picker needs a pre-join
      endpoint listing a room's seats, which is exactly the endpoint colour wanted and couldn't
      justify alone; colour rides along on it and is picked with the seat
- [ ] The preset palette itself — has to dodge colors that already carry meaning on the canvas
      (amber is both the erase highlight and today's single hardcoded ping color, sky blue is the
      measuring tool, red is the fog-hide preview) and stay legible on both light and dark maps,
      the way
      [room-member-dark-map-stroke-palette](../user-stories/room-member-dark-map-stroke-palette.md)
      already had to solve for drawings
- [ ] Whether color reaches any UI beyond pings and chat names

## Related user stories

- [room-member-identity-color](../user-stories/room-member-identity-color.md)
