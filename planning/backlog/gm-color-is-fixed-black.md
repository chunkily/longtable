---
title: The GM's colour is a fixed black
created: 2026-08-19
status: done
tags: [identity]
---

The GM picked an identity colour like everyone else — from the sixteen in
[identity-color](identity-color.md), on the create-room form and again from `Seats`. That was
asking a question with one sensible answer: there is exactly one GM at a table, everyone already
knows which name is theirs, and the colour spent on them was one fewer for the people it actually
distinguishes.

Black instead, and not chosen.

## What shipped

`GM_IDENTITY_COLOR` in `web/src/lib/identity-color.ts`, and `seatHex(seat, scheme)` beside it —
what anything drawing a seat's colour now calls, since the answer depends on the *role* rather
than only on the stored key. It reaches the three places a colour shows: the name in chat, the
ping ring on the map, and the swatch in `Seats` and the rail.

Nothing offers a GM a choice any more: the create-room form asks for a name and a password, the
GM step of the join form skips the palette, `Seats` renders no palette for them, and
`participant.setColor` is refused over the socket.

Decisions worth not rediscovering:

- **It's a light/dark pair, not a hex.** `#000000` on the light scheme and `#ffffff` on the dark
  one — the same two the drawing palette pairs. A GM's name in chat is DOM text on a themed panel,
  so one fixed hex is unreadable in one of the two schemes, and that is not a corner case: it's
  half the table's browsers.
- **The ping is the one place the scheme is a guess**, and it's accepted rather than solved. A
  ping is painted on map art, which has no idea what the page is wearing (`stroke-colors.ts` has
  the long version) — so a GM pinging a dark map under a light UI gets a black ring on dark art.
  The fix would be a halo behind every ping, which is a bigger change than the colour it repairs.
- **The role decides it where it's drawn; nothing is stored and nothing was migrated.**
  `CreateRoom` and `GMLogin` lost their colour parameter outright rather than being passed `""` by
  their callers, so a GM seat cannot be given one — that was a mechanical edit across ~20 test
  files and is why this change's diff is wider than it reads. A room made before this still has a
  key in the GM's row; `seatHex` ignores it. Two answers to "what colour is the GM" is exactly
  what the parameter removal prevents.
- **`participant.setColor` refuses a GM rather than storing and ignoring it**, for the same
  reason.
- **The rail's swatch is a plain dot for a GM**, not a button. It exists as the way in to the
  palette; a control that opens a dialog to show you no control is worse than no control. `Seats`
  is still in the menu, which is how a GM reads the roster.

## Related user stories

- [room-member-identity-color](../user-stories/room-member-identity-color.md) — now a Player's
  story rather than a Room Member's; see the note at its foot.
