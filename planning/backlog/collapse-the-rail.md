---
title: Collapse the side rail on a large screen too
created: 2026-10-29
status: open
tags: [ui, layout, tablet]
---

The rail down the right is fixed open at `lg` and up.
[full-bleed-map-layout](full-bleed-map-layout.md) decided that — "Static — it does not collapse" —
on the grounds that a collapsible rail needs something left on the map to bring it back, and the
map was still about 80% of the screen without it. That was a judgement made without a tablet in
hand. Using one at the table, the GM found the rail is the thing they most want out of the way: a
landscape tablet is `lg` wide, so it gets the full desktop rail, and on a 10-inch screen that rail
is a far bigger share of the map than 20% feels like.

So: a way to fold the rail away at every width, and the "something left behind" the original
decision priced in, accepted as worth paying for.

- [ ] A collapse control on the rail itself, and **something on the map to bring it back** that
      can't be mistaken for a tool. A small tab on the right edge is the obvious shape; a toggle
      in the toolbar's second cluster is the alternative, and keeps the map edge clear
- [ ] Collapsed means the map gets the width — the stage resizes to fill it rather than the rail
      merely sliding over the top. Check the resize path the bottom sheet already relies on
- [ ] Remembered per browser, like the theme and the bold grid — it's a question about this screen
- [ ] Nothing the rail carries becomes unreachable while it's folded. The selected token's panel
      is the one to think about: selecting a token with the rail away could open it again, or
      leave it be and rely on the hover card. Decide which rather than finding out
- [ ] The reconnect banner, which is on the map rather than in the rail, needs no change — but
      check the "you're not on the table's scene" notice, which *is* in the rail, isn't lost by
      folding it away

Below `lg` the rail is already a bottom sheet with its own way of getting out of the way; this item
is about the widths that never had one. Whether the sheet's behaviour and this one should end up
as the same control is worth a look once both exist.
