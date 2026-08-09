---
title: Room Member uses one side panel for chat, initiative and room actions
created: 2026-08-04
status: done
---

As a Room Member
I want chat, the initiative tracker and the room's actions in one panel down the side of the screen
So that I can reach all of them without the screen filling up with separate boxes

## Acceptance criteria

- [ ] The panel runs the full height of the window down the right, and does not collapse
- [ ] It holds the selected token at the top, session info under it, and chat or the initiative
      tracker filling the rest
- [ ] The selected-token area keeps its height when nothing is selected, so the rest of the panel
      doesn't jump when I click empty map
- [ ] Three icons at the foot of the panel switch between chat, the initiative tracker, and a menu
- [ ] Switching between chat and the tracker doesn't lose an in-progress chat draft or tracker
      state
- [ ] The menu offers Scenes, New scene, Assets, Manage room and Leave room
- [ ] Assets takes me to the assets page rather than trying to fit it into the panel

## Still open after full-bleed-map-layout (2026-08-07)

The panel itself is built and everything structural holds: full height, doesn't collapse, selected
token at the top holding its height, session info under it, three foot icons, and a chat draft
that survives switching to the other panel and back (both stay mounted and are hidden with CSS
rather than swapped out). Assets links out to `/r/{slug}/assets` rather than folding into the rail.

Two criteria don't hold literally, so this stays `incomplete`:

- **"chat or the initiative tracker filling the rest"** — the tracker is a placeholder that says it
  isn't built yet. It's blocked on [initiative-tracker](../backlog/initiative-tracker.md); the
  switcher exists so that feature lands as contents rather than as a third icon nobody notices.
- **"The menu offers Scenes, Assets, Manage room and Leave room"** — it offers those four plus
  **New scene**, because the backlog item's toolbar section says both Scenes and New scene leave
  the toolbar and live under the menu. That's a conflict between this story and its item rather
  than something unbuilt: rewrite this criterion to five entries when someone confirms the menu is
  right, and note that `Manage room` is currently an empty container for three settings that are
  each still their own open item.

## Done 2026-08-09

Both of those are settled, so this is `done`.

[initiative-tracker](../backlog/initiative-tracker.md) shipped, and the second criterion now holds
as written: the placeholder is gone and the panel switches between chat and a real turn order,
with the "doesn't lose state" criterion holding for the tracker too — both panels still stay
mounted and hidden with CSS, so a half-typed combatant survives a trip to chat and back.

The menu criterion is **rewritten to five entries** rather than left as a marker. The five have
been on screen since 2026-08-07 and the backlog item asked for them explicitly; the story was the
document that was wrong, not the code. `Manage room` is no longer an empty container either — it
holds seats and the movement lock, with room privacy and deleting a room still open.
