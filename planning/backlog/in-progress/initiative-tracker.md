---
title: Initiative tracker
created: 2026-07-29
tags: [combat, ui]
---

Add an initiative tracker: GM-managed turn order with entries that can either link to an
existing map token or stand alone (name + value, for things like lair actions or hazards). Full
turn tracking — current turn highlighted, next/previous navigation, round counter.

Surfaced as a tab alongside chat and event logs — see [chat-panel-tabs](chat-panel-tabs.md).

[token-selection-highlight](../open/token-selection-highlight.md) wants clicking a tracker entry
to select its linked token, and [token-recent-interaction-stacking](../open/token-recent-interaction-stacking.md)
wants that same selection to bump the token to the top of the canvas stack; both are blocked on
this item shipping first.

## Related user stories

- [gm-add-initiative-entry](../../user-stories/gm-add-initiative-entry.md)
- [gm-edit-initiative-order](../../user-stories/gm-edit-initiative-order.md)
- [gm-remove-initiative-entry](../../user-stories/gm-remove-initiative-entry.md)
- [gm-advance-initiative-turn](../../user-stories/gm-advance-initiative-turn.md)
- [room-member-view-initiative-tracker](../../user-stories/room-member-view-initiative-tracker.md)
- [gm-clear-initiative-tracker](../../user-stories/gm-clear-initiative-tracker.md)
