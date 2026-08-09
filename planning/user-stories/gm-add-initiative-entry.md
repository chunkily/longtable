---
title: GM adds an initiative entry
created: 2026-07-29
status: done
---

As a GM
I want to add a combatant to the initiative tracker, either by linking an existing token or entering a freestanding name and value
So that I can build the turn order for an encounter, including things that aren't on the map as tokens (e.g. lair actions, hazards)

## Acceptance criteria

- [ ] I can add an entry by selecting an existing token, which pulls in its name/image automatically
- [ ] I can add a freestanding entry with just a name and an initiative value, with no token required
- [ ] I can mark a freestanding entry as hidden, independent of any token's visibility
- [ ] Each entry has an initiative value used to determine turn order
- [ ] New entries are inserted into the tracker in the correct sorted position

## Verified 2026-08-09

All five hold. The first reads "pulls in its name/image automatically", and it is stronger than
that in practice: a linked entry *resolves* its name and art from the token every time the tracker
is sent rather than copying them once, so renaming a token renames its entry for the whole room.

The third criterion — hiding a freestanding entry — is deliberately not offered for a linked one:
that entry's visibility is its token's, and two switches for one answer is how they end up
disagreeing.
