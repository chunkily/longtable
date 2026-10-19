---
title: Tokens made from an icon and a colour
created: 2026-10-06
status: open
tags: [tokens, canvas, protocol, playtest]
story: room-member-makes-a-token-without-art
---

Playtest feedback: not everyone prepares art, and a disc with two initials on it doesn't say
"goblin". Shipping a library of raster art isn't feasible yet; a pool of icons is a few hundred
bytes of SVG apiece.

Most of this already exists in a cruder form. A token without art is drawn by
`addTokenPlaceholder` as a disc in `colorFor(token.id)` — a hash of the id — with the first two
letters of its name in white. This makes the colour a choice and lets the letters be an icon.

**Data.** Two keys on `token`, `icon` and `color`, added through `addMissingColumns` as
`TEXT NOT NULL DEFAULT ''` so every existing token renders exactly as it does today. Keys rather
than SVG or hex, the way a seat stores its colour key, each validated in Go against a fixed list.
What a token shows, in order: its art, else its icon, else its initials. **An unpicked colour stays
the hash colour** — colour already means *who* in chat and pings, and defaulting a token to its
owner's seat colour was considered and decided against: a token's colour stays decoupled from
identity unless someone deliberately picks one.

**Icons.** A curated pool of forty to sixty from Lucide, which is already a dependency (ISC) and
matches the toolbar — sword, shield, skull, ghost, crown, flame, wand, bow, axe, castle, tent,
chest, cat, dog, bird, rat, spider, bone, gem, key, scroll, eye and the like — each with search
words, so "undead" finds the skull and the ghost. **Keys are namespaced** (`lucide:sword`) so that
[game-icons-second-icon-pool](game-icons-second-icon-pool.md) is purely additive. The token
dialog's art picker gains an Art/Icon choice.

**Colours.** The sixteen identity colours, as the same swatches. A white glyph vanishes on the light
ones (Lantern Yellow, Honey Gold, Lagoon Teal, Frost Cyan, Bone White), so each palette entry also
records whether a glyph on it is dark or light — next to the hex, not computed at render. Keep
`TestIdentityColors_MatchTheClientPalette` (or a sibling) covering whatever Go now needs to know.

**A two-tone ring on every disc token.** A coloured disc blends into map art just as easily as a
glyph blends into its disc. A single border colour only moves the problem (white on a snow map,
black on a dungeon floor), so this is the bold grid's answer — a dark band over a pale casing, one
fixed pair whatever the disc colour or the page scheme, with a comment pointing at
`GRID_LINE_BOLD` for why. It sits *inside* the token's edge, so a token never reads larger than its
square and a solid ring inside never gets mistaken for the dashed selection ring 6px outside.
Thickness is a fraction of the radius with a screen-pixel floor. Initials discs get it too — on the
map they are the same object as an icon disc, and drawing one without it would read as a second
kind of token. Art tokens do not: a coloured ring around art was considered and left for later.

**Everywhere token art appears.** The map, initiative panel, details panel, hover card and dialog
preview all draw art today, and each now has to draw an icon. One shared avatar component for the
DOM places, rather than five copies; the canvas draws the same thing. The DOM avatars skip the ring,
since they sit on a themed panel rather than on map art. Initiative entries "take the token's name
and art" and so take its icon and colour too.

**Permissions** are art's: a GM on any token, a Player on their own (ADR-0007 untouched).

- [ ] `icon` and `color` in `token-fields.ts`, so undo's "still how I left it?" and `updateToken`'s
      no-op guard both see them
- [ ] A batch of tokens (`Monkey 1`…`Monkey 8`) all carry the chosen icon and colour
- [ ] Unknown icon or colour key refused by the hub with an error the client can show
- [ ] `ws-protocol.md` updated for the new `token.create`/`token.update` fields
- [ ] The icon pool's labels and search words, and every new string, go through `longtable-copy`

- [ ] `icon` and `color` are identity fields, so `handlePartyAdd` in `internal/ws/party.go` copies
      them onto each new party token alongside name and art (it lists exactly what it copies)

Out of scope: a coloured ring around art tokens, and defaulting a token to its owner's colour.
