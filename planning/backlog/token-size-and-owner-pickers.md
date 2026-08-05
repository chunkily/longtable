---
title: Token size and owner pickers in creation dialog
created: 2026-07-29
status: done
tags: [tokens, ui]
---

Add a size picker and an owner picker to `create-token-dialog.svelte`. Broadened from
owner-only: both are additions to the same dialog (which already has name, art via the asset
library, and visibility), likely to ship in the same pass.

- **Size**: **the control already exists** —
  [token-detail-panel](token-detail-panel.md) needed the same one and built it as
  `web/src/lib/components/token-size-picker.svelte`, with a `squares` bindable and a
  `sizeForSquares` helper for reading a stored token back. Wiring it into
  `create-token-dialog.svelte` and passing `width`/`height` on `token.create` is all that's left
  here; the backend already accepts them and defaults to 1×1 when omitted. Note it offers four
  options rather than six: Tiny, Small and Medium are all 1×1, and three options meaning the same
  thing can't round-trip when a token is read back, so they share one option carrying all three
  names.
- **Owner**: **no longer blocked.** `handleTokenCreate` has always accepted
  `ownerParticipantId`, and [list-room-participants](list-room-participants.md) has since
  shipped the candidates: `client.participants` is the whole roster — everyone who has ever
  joined, which is the right list here rather than `connectedParticipants`, since a GM prepping
  tokens before a session is assigning them to people who aren't online yet. All that's missing
  is the control. `token.update` will take an owner the same way once it has a field for it (a
  line in the request struct and a line in the assignment block — see
  [token-detail-panel](token-detail-panel.md)).

## What shipped

Both pickers are on `create-token-dialog.svelte`, and the owner picker is on the edit dialog too —
which had the size one already. A token can be born Large and belonging to Bob instead of being
created 1×1 and unowned and then edited twice. The details strip above chat shows whose token it
is, to everyone rather than only the GM, resolving the stored id through `client.participants`.

The size picker went in as-is, as the item predicted. Three things it didn't:

- **`ownerParticipantId` was never scoped to the room.** `handleTokenCreate` has always accepted
  the field and passed it straight to the store, where the only check was a foreign key to
  `participant(id)` — a global table. A GM could have owned a token to somebody in another room.
  Nothing in the UI could do it, which is presumably why it survived; adding a control that sends
  the field is what made it reachable. `requireOwnerInRoom` is now the participant twin of
  `requireAssetInRoom`, on both create and update, backed by `store.ParticipantInRoom`. Same
  reasoning as assets: an unguessable ID is not a scoped one.
- **`token.update` had no owner field at all**, so the edit dialog couldn't reassign one. Adding
  it flipped a documented behaviour: `TestTokenUpdate_PreservesFieldsItDoesNotCarry` asserted the
  owner survived an update that omitted it, which was true only because nothing could set it.
  Now the owner follows the rule the other editable fields follow — sent every time, omitted
  means cleared — because taking a token back off a Player is a real edit and the wire can't tell
  "left alone" from "unassign". The test now asserts the opposite and says why. **A client that
  sends a partial `token.update` will silently unassign the owner.**
- **The owner picker is a `<select>`**, not the row of buttons the size and visibility pickers
  use. Those have four options and two; a room has as many members as it has people, and a button
  per player stops working somewhere around a dozen. It offered `participants` — the whole roster —
  rather than `connectedParticipants`, because a GM prepping an encounter the night before is
  assigning tokens to people who are all offline.

  **That last part was reversed on review, and the reason it was wrong is worth keeping.** A
  participant row is created on *every join*: the same person from a phone as well as a laptop,
  and anyone who ever cleared their browser storage, is a separate row forever. A room that has
  run for a few months offers a dozen names to choose four from, several of them the same person.
  Who is at the table is the question a GM is actually answering, so `ownerOptions` in
  `web/src/lib/token-owner.ts` builds the list from `connectedParticipants`.

  The prepping-the-night-before case is real but rarer, and the answer to it is to hand the token
  over when people arrive — which the edit dialog already does.

  **GMs aren't offered either**, settled in the same review. A GM owning a token would grant them
  nothing: they may move any token whatever [the ownership
  lock](token-move-ownership-lock.md) ends up saying, and may already edit any token's HP.
  The only thing it could express is "this is my character", and an option that looks like a
  permission but confers none is worse than no option. That makes the empty list reachable — a GM
  setting up before anyone arrives — so the picker says "No players are connected" rather than
  showing a control whose only entry is "Nobody".

  **`ownerOptions` keeps a token's current owner on the list whoever they are, and that is not a
  nicety.** Since `token.update` sends every editable field every time, a list that dropped the
  current owner would leave the select with no matching option, the browser would fall back to
  the first one — "Nobody" — and a GM renaming the token would silently take it off them. It
  catches both ways of falling off the list: a Player who went offline, and a GM holding a token
  from before GMs stopped being offered. The `online` flag means *connected*, not *offered*, so a
  kept GM sitting right there isn't labelled absent. An e2e case closes a player's browser and
  renames their token specifically to catch a regression here.

  **The server still validates room membership, not connection**, and deliberately: being offline
  isn't being gone, and a rule keyed on presence would refuse an assignment the moment someone's
  socket blipped. The picker is narrower than what the protocol accepts, which is the right way
  round.

This unblocks [token-move-ownership-lock](token-move-ownership-lock.md), which needed
owners to be assignable before an owners-only movement rule could mean anything.

## Related user stories

- [gm-set-token-size](../user-stories/gm-set-token-size.md)
- [gm-assign-token-owner](../user-stories/gm-assign-token-owner.md)
