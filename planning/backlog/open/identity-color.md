---
title: Room Member identity color
created: 2026-08-03
tags: [identity]
story: room-member-identity-color
---

A preset color, chosen per room, shown next to pings and chat names. Deliberately scoped apart
from [first-time-display-name-prompt](first-time-display-name-prompt.md) — color is room-level,
display name is (eventually) device-level, so don't assume they share a picker or a mechanism.

Not yet decided, worth settling before picking this up:

- [ ] Whether this ships at all — still weighing whether it earns the wire surface below
- [ ] Where picking happens: on the join form (needs a new pre-join REST endpoint to expose live
      connected-player colors, since that state currently only exists in the hub once a socket is
      open) vs. after connecting (fits the existing WS command/event flow, but color starts unset
      — needs a fallback, e.g. gray, for anyone who hasn't picked yet or joined before this
      shipped)
- [ ] The preset palette itself — has to dodge colors that already carry meaning on the canvas
      (amber is both the erase highlight and today's single hardcoded ping color, sky blue is the
      measuring tool, red is the fog-hide preview) and stay legible on both light and dark maps,
      the way
      [room-member-dark-map-stroke-palette](../../user-stories/room-member-dark-map-stroke-palette.md)
      already had to solve for drawings
- [ ] Whether color reaches any UI beyond pings and chat names

## Related user stories

- [room-member-identity-color](../../user-stories/room-member-identity-color.md)
