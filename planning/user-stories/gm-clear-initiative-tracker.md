---
title: GM clears the initiative tracker
created: 2026-07-29
status: done
---

As a GM
I want to clear the initiative tracker
So that I can start fresh for a new encounter without manually removing every entry one by one

## Acceptance criteria

- [ ] A single action removes all entries and resets the round counter and current turn
- [ ] Clearing the tracker does not affect the underlying tokens on the map, only their entries in the tracker

## Verified 2026-08-09

Both hold, and the second is asserted directly — the token is still readable from the store after
a clear, and still offered by the panel's own combatant picker.

The entries and the round reset in one transaction: a half-cleared tracker showing round 7 of an
encounter that no longer exists is worse than either half on its own. In the UI it takes two
clicks, like removing a seat — it is the one action in this panel that the other button can't
undo.
