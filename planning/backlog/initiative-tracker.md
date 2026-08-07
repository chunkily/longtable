---
title: Initiative tracker
created: 2026-07-29
status: open
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
