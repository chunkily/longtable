---
title: Initiative tracker
created: 2026-07-29
status: done
tags: [combat, ui]
---

Add an initiative tracker: GM-managed turn order with entries that can either link to an
existing map token or stand alone (name + value, for things like lair actions or hazards). Full
turn tracking — current turn highlighted, next/previous navigation, round counter.

Surfaced alongside chat in the room's side panel. That surface changed on 2026-08-04:
[chat-panel-tabs](chat-panel-tabs.md) was superseded unshipped by
[full-bleed-map-layout](full-bleed-map-layout.md), where the panel is a full-height rail
and the switch is three icons at its foot rather than a tab strip. Nothing about the tracker itself
changes — it's the same content in the same region, reached a different way — but which of the two
lands first decides whether this item builds a temporary home for the tracker or drops straight
into the new one.

**That's settled: the layout landed first (2026-08-07), so the home already exists.** The rail's
second panel is an `initiativePanel` snippet in `web/src/routes/r/[slug]/+page.svelte` that
currently says the tracker isn't built yet, reached by the Swords icon at the foot of the panel.
Replacing that snippet's body is the whole of the UI surface — the switcher, the mobile sheet copy
and the "switching panels doesn't lose state" behaviour are all done and tested. Both panels stay
mounted and are hidden with CSS rather than swapped out, deliberately, so whatever the tracker
holds survives a trip to chat and back without needing to be lifted into page state.
[room-member-room-side-panel](../user-stories/room-member-room-side-panel.md) is `incomplete`
naming this item as the reason.

[token-selection-highlight](token-selection-highlight.md) has **shipped** everything
except its last checkbox — clicking a tracker entry to select its linked token — which is waiting
on this item and is a one-liner once entries exist: set `selectedTokenId`, the `$bindable` prop
the room page already owns. [token-recent-interaction-stacking](token-recent-interaction-stacking.md)
wants that same selection to bump the token to the top of the canvas stack, and is still blocked
on this item shipping first.

## Related user stories

- [gm-add-initiative-entry](../user-stories/gm-add-initiative-entry.md)
- [gm-edit-initiative-order](../user-stories/gm-edit-initiative-order.md)
- [gm-remove-initiative-entry](../user-stories/gm-remove-initiative-entry.md)
- [gm-advance-initiative-turn](../user-stories/gm-advance-initiative-turn.md)
- [room-member-view-initiative-tracker](../user-stories/room-member-view-initiative-tracker.md)
- [gm-clear-initiative-tracker](../user-stories/gm-clear-initiative-tracker.md)

## What shipped

All six stories, as `initiative_entry` plus two columns on `room`, six GM-only commands in
`internal/ws/initiative.go`, and `web/src/lib/components/initiative-panel.svelte` filling the
`initiativePanel` snippet the layout left behind.

**The tracker belongs to the room, not the scene**, and that is the decision everything else
follows from: a GM flipping to the battle map mid-fight must not lose the encounter. Entries hang
off `room_id`; the turn and round are two columns on `room` rather than a table of their own,
since a room runs one encounter at a time and a second table would be a join for two scalars.

**Every change broadcasts the whole tracker.** Entries are withheld per recipient, a turn advance
moves the round as well as the pointer, and a removal can move whose turn it is — so most "small"
changes are several fields anyway, and a delta protocol would buy nothing but reconciliation bugs.
A tracker is a couple of dozen rows that change once a turn.

**An entry is either a token or a name, and a linked one resolves live.** Name and art are read
from the token on every send rather than copied at creation, so renaming a token renames its
entry. Three consequences worth not rediscovering:

- Any change to a token has to re-broadcast the tracker if an entry stands for it
  (`broadcastInitiativeIfLinked`), or a renamed token keeps its old name in the order and a
  revealed one never appears.
- `handleTokenDelete` must ask **before** deleting: `initiative_entry.token_id` is
  `ON DELETE CASCADE`, so afterwards there is no entry left to notice. That's `tokenIsInInitiative`,
  which exists for that one ordering problem.
- A hidden token's entry is withheld from Players by reading *the token*, not a copy — so the two
  answers can't drift apart. A freestanding entry has its own flag, which is the only thing the
  eye button toggles.

**The turn arithmetic is in `advanceTurn`, away from the handler**, because it is all edge case:
the first press of Next with nobody up (start at the top, don't count a round nobody played), the
wrap in each direction, and a round counter floored at 1. The rule is that the round changes
*only* at the wrap, which is what makes next-then-previous land exactly where it started across a
round boundary — an off-by-one there is a table arguing about whether a spell has expired.

**Reordering is only among ties**, and refuses to cross an initiative value. The order *is* the
values; letting an entry jump a higher roll would make the list disagree with the numbers printed
beside it. It renumbers `sort_order` across the whole list rather than swapping two — every new
entry starts at 0, so a swap would trade one zero for another and do nothing.

Two things found while building it:

- **The panel's name field is labelled `Call it`, not `Name`.** Both side panels stay mounted, so
  this box shares the page with every dialog the room can open, and Playwright's `getByLabel`
  matches on a *substring* — a second "Name" anywhere made `getByLabel('Name')` ambiguous in
  fifteen existing specs at once. The first full run after adding the panel failed six specs at
  `Create scene`.
- **`token-selection-highlight`'s last checkbox is done**: clicking a linked entry sets
  `selectedTokenId`. It was a one-liner, exactly as that item predicted, and it is covered here
  rather than there.

Not built, and not asked for: rolling initiative for you. `initiative.add` takes the number, and
`/roll 1d20+2` in chat is where it comes from. Renaming a freestanding entry isn't offered either
— the value, the hidden flag and the manual order are editable in place, and a rename is a remove
and an add.

## Unblocks

- [token-recent-interaction-stacking](token-recent-interaction-stacking.md) was waiting on this
  shipping so a tracker click could bump a token up the canvas stack. That click now exists and
  sets `selectedTokenId`.
