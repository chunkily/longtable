---
title: GM removes an initiative entry
created: 2026-07-29
status: done
---

As a GM
I want to remove an entry from the initiative tracker
So that I can take defeated or fled combatants out of the turn order

## Acceptance criteria

- [ ] I can remove any entry, token-linked or freestanding
- [ ] If it's currently that entry's turn, removing it automatically advances to the next entry
- [ ] The change is visible to all Room Members in real time

## Verified 2026-08-09

All three hold. The second needed care in the ordering rather than in the logic: whoever is next
has to be worked out *before* the row goes, since afterwards there is nothing left to be next of.

There is a second way an entry can leave — deleting its token, which cascades — and that one
can't hand the turn on, because the cascade happens in SQLite where nothing is watching. A turn
pointer left dangling reads as "nobody's turn" (`GetInitiativeState` validates it against the list
it just read) and one click on Next recovers it.
