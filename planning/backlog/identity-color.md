---
title: Room Member identity color
created: 2026-08-03
status: done
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
**That shipped on 2026-08-07, so nothing blocks this any more.** What it left you:
`GET /api/rooms/{slug}/seats` (`listSeats` in `internal/api/rooms.go`, `$lib/api.ts`) already
returns a seat's name, role and whether anyone is on it, and the seat picker on the join screen
already renders one button per seat — a colour is a column on `participant`, a field on that
payload, and a swatch on that button. Note the endpoint is deliberately unauthenticated and
deliberately thin, so adding to it is a decision about what a stranger with the link may see, not
a free extension.

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

## What shipped

All four open questions above, answered:

**It ships.** Six colours — violet, indigo, teal, emerald, lime, pink — picked on the same form as
a name, on every path that makes a seat: creating a room, taking a new chair, a GM setting one out
in `Manage room`.

**The palette dodges what the canvas already says.** Amber is the erase highlight, sky blue the
measuring tool, red the fog-hide preview, and a unit test asserts none of the six is one of those.
All six are mid-tone and saturated rather than pastel or deep, which is the same light-map/dark-map
constraint [dark-map-stroke-palette](dark-map-stroke-palette.md) is still open about for drawings.

**Colour reaches the name in chat and the colour a ping pulses in** — the two the story named — plus
a swatch on the seat picker, which is not decoration: "see which colours are taken before choosing"
can only be answered on the screen that exists before joining.

Decisions worth not rediscovering:

- **A seat stores a palette key, never a colour.** `violet`, not `#8b5cf6`. The value ends up in a
  `style` attribute on every other client's screen, and a column that takes any string is a CSS
  injection waiting for somebody to try it — ADR-0007 trusts the table with the room's contents,
  not with arbitrary text in someone else's stylesheet. `rejectUnknownColor` guards every write.
  Keys also mean retuning a shade is a client-side edit rather than a migration.
- **The palette therefore lives twice**, in Go and in TypeScript, and
  `TestIdentityColors_MatchTheClientPalette` reads the `.ts` file to keep them identical. A key with
  no hex renders as no colour at all — silently, for one person, on a screen no Go test looks at.
  Checked by renaming a key and watching it fail.
- **A ping carries `participantId`, not a colour.** The colour is looked up in the roster every
  client already has, so there is no second copy to go stale. Same for a chat name.
- **The suggestion happens when the form opens, not in an effect.** The first version recomputed
  "first unused colour" whenever the seat list changed, which would quietly overwrite a colour
  somebody had just picked on the form it was helping with.
- **Adding `color` to `GET /seats` tripped the guard test that keeps that endpoint thin**, which is
  exactly what the item warned about. It was widened deliberately, with the reasoning in the test:
  a colour beside a name a stranger could already read, in exchange for the only moment the choice
  is any use.
- The e2e helper matches a swatch loosely, because a colour someone already has is announced as
  "Pink, taken" — matching exactly finds it for the first taker and then silently stops, costing a
  full test timeout instead of a clear failure.

## Correction, same day: sixteen colours, and names worth saying

The palette above shipped as six, chosen to dodge the colours the canvas already speaks with and
to stay legible on any map. **Both constraints were dropped on the GM's call**, and the palette is
now sixteen: Blood Red, Sunset Orange, Honey Gold, Lantern Yellow, Swamp Olive, Forest Green,
Lagoon Teal, Frost Cyan, Royal Blue, Midnight Indigo, Arcane Violet, Dusk Magenta, Rose Pink, Rust
Brown, Storm Grey, Bone White.

The reasoning that replaced it: six colours means two people match by the fourth arrival, and
neither clash the old rule was avoiding is actually ambiguous on screen — a ping is a pulsing ring
where the erase halo is a static outline, and the fog preview only ever appears under the GM's own
cursor mid-drag. Contrast against a particular map is the chooser's problem now, the same trade the
drawing palette makes. The unit test that enforced the old rule was deleted rather than adjusted,
with a comment where it stood saying why, since it would otherwise have been enforcing a decision
nobody holds.

**A key is the base colour alone** — `red` for a swatch called Blood Red — rather than the whole
name. The modifier is the half most likely to be reworded, and keying on the base means rewording
it touches no stored data at all. It also forced one naming decision: Swamp Olive is an olive
rather than a second green, because two greens would have collided on one key, and a key collision
is the one thing this scheme can't absorb.

Seats holding a key from the first six still read correctly where the key survived (`violet`,
`indigo`, `teal`, `pink` — in a new shade), and read as unchosen where it didn't (`emerald`,
`lime`). That is `identityHex` returning null for an unknown key, which is the behaviour that makes
changing a palette safe at all.
