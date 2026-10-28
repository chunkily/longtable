---
title: Room Member makes a recognisable token without art
created: 2026-10-06
status: done
---

As a Room Member
I want to make a token from an icon and a colour I pick
So that I can put something recognisable on the map without having prepared any art

## Acceptance criteria

- [ ] I can give a token an icon instead of art, chosen from a list I can search by what I'm
      looking for ("undead", "beast"), not only by the icon's name
- [ ] I can pick the token's colour from the same colours seats use
- [ ] The icon is always readable on whatever colour I picked
- [ ] An icon token stands out from the map whatever the map art looks like
- [ ] The token looks the same on the map, in the initiative tracker and in its details
- [ ] Tokens I made before this keep the colour they had

## Notes

The last criterion first read "look exactly as they did", and was rewritten when this shipped
(2026-10-28) because two deliberate changes made it untrue. A token without art now has the
two-tone ring every disc token gets, so it stands out from the map the way an icon token does — a
second kind of disc without it would have read as a different sort of token. And its initials
are drawn in the ink that reads on its colour, where they were always white and disappeared on
the yellow half of the wheel. Its colour is unchanged. [icon-tokens](../backlog/icon-tokens.md)
has the detail.

From the first playtests (October 2026). A library of ready-made raster art was what people asked
for first and is out of reach for now, since it would all have to be distributed with the binary.
