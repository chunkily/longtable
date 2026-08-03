---
title: GM assigns a token's owner
created: 2026-07-29
---

As a GM
I want to assign a Room Member as the owner of a token when I create it
So that ownership-based permissions (movement, HP editing) have someone to apply to

## Acceptance criteria

- [ ] Token creation lets me pick an owner from the room's current members, or leave it unowned (e.g. for NPCs/monsters)
- [ ] A token's owner is visible to Room Members, so it's clear whose token is whose

## Reading taken

"The room's current members" was read as the **Players connected right now**, narrower than the
literal Room Member in the goal above, on two counts:

- *Current*, not the roster. A participant row is created on every join, so the roster is every
  person who has ever been here — several of them the same person from a second browser.
- *Players*, not GMs. The benefit line is the reason: ownership exists so that movement and HP
  permissions "have someone to apply to", and neither can apply to a GM, who may move any token
  and edit any token's HP regardless. Offering it would be a control that looks like a permission
  and grants nothing.

A token that already has an owner keeps them on the list either way, so neither narrowing can
silently reassign one. See
[token-size-and-owner-pickers](../backlog/done/token-size-and-owner-pickers.md).
