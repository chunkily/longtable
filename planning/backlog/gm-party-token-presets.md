---
title: GM flags tokens as the party, and stamps them into a scene
created: 2026-08-14
status: open
tags: [tokens, scenes]
story: gm-add-party-to-scene
---

Every token is scene-scoped (`token.scene_id`, cascade-deleted with its scene — see
[scene-management](scene-management.md)), so getting the same handful of player-character tokens
onto a new scene means re-creating them from scratch: name, art, size, owner, all typed in again.
That's exactly the kind of thing a GM does at the start of every session.

Let a GM flag a set of existing tokens as "the party" (a boolean on the token, GM-only to set,
same shape as the existing visibility flag). Any scene — new or existing — gets an "Add party"
action that copies fresh token instances (name, art, size, owner; not HP/conditions/position,
which are per-fight state, not identity) onto it, spread over free squares the same way a batch of
new tokens spreads today (`token.create`'s numbering/spread logic).

## Open questions for whoever picks this up

- Does flagging a token "party" travel anywhere, or does the flag simply live on the token row and
  vanish with it if that token is deleted? (Simplest: the latter — nothing else in the data model
  survives a token's deletion either, so there's no reason this flag should.)
- "Add party" duplicates the *current* state of each flagged token's identity fields at the moment
  it's clicked — it is not a live link back to them. Editing a flagged token afterward doesn't
  retroactively change copies already stamped onto other scenes. This is the simpler of two models
  discussed (the other being a template distinct from any live token) and is the one to build
  unless a concrete need for the other shows up.

## Related user stories

- [gm-add-party-to-scene](../user-stories/gm-add-party-to-scene.md)
