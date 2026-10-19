---
title: GM flags tokens as the party, and stamps them into a scene
created: 2026-08-14
status: done
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

If [icon-tokens](icon-tokens.md) has shipped by the time this is picked up, `icon` and `color` are
identity fields like art and belong in the copy — not per-fight state.

## What shipped

A GM marks a token `In the party` — in the edit dialog, or in New token when making one token —
and `Add party`, an icon past the toolbar's divider beside fog, puts the whole party on the scene
the GM is looking at, spread out from the middle of their view. Making a scene already takes the GM
to it, so a new scene is one click from having the party on it.

Both open questions above were answered, and the first one differently from the suggestion:

- **The party is a roster of characters, not a boolean.** A flag on the token breaks on the second
  use whichever way it goes: flag the copies too and `Add party` on the third scene copies
  everyone twice; don't and the party exists on one scene, gone the night that scene is deleted,
  with every copy's box unticked. So a party token carries a `party_key` and every copy shares it.
  `Add party` puts down one per key and skips keys already on the scene — a second press says
  `The whole party is already here.`, and pressing it again after someone new is marked adds just
  them. Unticking takes the character out on **every** scene (`store.LeaveParty`), since that is
  what a GM means by it.
- **Still a copy, not a live link**, as suggested — but it copies the *newest* copy of each
  character, by a new `updated_at` that create and edit set and a move doesn't. Copying the first
  one ever flagged would put old art back on every new scene.
- **A copy brings name, art, size and owner, plus visibility** — the last so a companion the GM is
  hiding doesn't walk into the next room in plain view. Trackers and conditions were offered and
  declined: each scene starts them blank.

Things that would cost time to rediscover:

- **`party.added` exists for undo.** `token.create`'s caller mints ids so it can record its own
  echoes; `party.add`'s can't, because how many characters arrive is the server's answer. The
  sender alone gets the ids after the `token.created` events and records one entry each.
- **`token.create` takes a `partyKey` from a GM**, so undoing a party character's deletion puts it
  back in the party. Undoing "take them out of the party" doesn't fully reverse it: it re-mints a
  key for that one token, and the copies on other scenes stay out. Accepted rather than solved.
- **`setTokenTrackers` carries `inParty`.** It rebuilds every field from the token, and leaving
  the new one out meant a GM changing a party character's hit points inline took them out of the
  party.
- **`spawnFootprints`** places tokens of mixed sizes, since a party mixes a Large companion with
  Medium characters; `spawnCells` now calls it. Unlike `spawnCells` it never puts a lone token on
  an occupied square, because nobody pointed at that square for it.
- **This found a bug that had nothing to do with the party.** `token.created`, `token.updated`,
  `drawing.created` and `ping` were folded in by `RoomClient` whatever scene they were for, so a
  GM prepping another scene put tokens, strokes and pings on every Player's map. Leaving the party
  touches copies on every scene, which would have made it much worse. Fixed first, as its own
  change; the correction is in [per-client-scene-viewing](per-client-scene-viewing.md).

- **The New token dialog scrolls now.** Its own comment said it was one field from the window's
  height, and the party choice was that field: on a 720px window it overflowed both ways, with the
  close button off the top and `Create token` off the bottom. It has the edit dialog's layout —
  capped to the window, fields scrolling, footer pinned. `asset-library.spec.ts` is what caught it.

Icons and colours from [icon-tokens](icon-tokens.md) haven't shipped, so they aren't copied yet —
the note above still applies to whoever builds that.

## Related user stories

- [gm-add-party-to-scene](../user-stories/gm-add-party-to-scene.md)
