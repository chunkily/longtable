---
title: Player-created tokens, and creating several at once
created: 2026-08-04
status: done
tags: [tokens, permissions]
---

Let a Player create their own tokens, and let anyone create several at once from one dialog.

The motivating case is a druid casting a third-level conjure spell and wanting eight monkeys on
the map. Today that's eight trips through the GM's dialog, and the GM is the only one who can make
them — so a Player's summons, familiars and companions are all busywork landing on the one person
who has the least spare attention during a fight.

`token.create` is GM-only today, and `token.delete` is GM-only *explicitly because* creation is
(see the note in `ws-protocol.md`). So this is a permission change on both, not just a new button.

## What a Player gets

- [ ] The same `New token` button the GM has, from the same place in the toolbar.
- [ ] **Owner is the creator, not a picker.** A Player making a token owns it; there's nothing to
      choose. This is also what finally gives [token-move-ownership-lock](token-move-ownership-lock.md)
      something to lock — ownership has been recorded and enforced nothing since it shipped.
- [ ] **No visibility control.** Hiding a token from the room is a GM power, and it's the one
      field a Player could use to hide something from the GM.
- [ ] **A Player can delete their own tokens.** Without this the eight monkeys become the GM's
      cleanup, which is the busywork being removed. The rule becomes "a GM, or the owner", rather
      than GM-only.

So the Player's dialog differs from the GM's by exactly two fields: the owner picker and the
visibility toggle.

## The count control

- [ ] A `How many` stepper, **for the GM as well as the Player** — a GM wanting six goblins has
      the same problem.
- [ ] **Capped at 20**, and the cap is enforced **on the server**. A stepper is a convenience, not
      a permission; the command has to refuse 500 whatever sent it.
- [ ] Naming: a count of one creates `Monkey`. A count above one creates `Monkey 1` … `Monkey 8`.
      No suffix is added unless there's more than one — otherwise every single token a GM makes
      picks up a pointless ` 1`.
- [ ] **They spawn as a block, not a stack.** Eight tokens on one square would be unusable, and
      [token-recent-interaction-stacking](token-recent-interaction-stacking.md) already records how
      hard it is to get at what's underneath. They should spread outward from the spawn cell into
      free squares.

## Undo

- [ ] **One undo per token, newest first.** Creating eight monkeys puts eight entries on the stack;
      Ctrl+Z removes Monkey 8, then Monkey 7, and so on.

Undoing the whole batch in one press was considered first and deliberately dropped. It would have
made this the first *grouped* action on the history stack, and the flat per-action history is a
property worth keeping — it's the same reasoning that left an eraser sweep as one undo per stroke
erased (see [undo-redo-drawing](undo-redo-drawing.md)). One-by-one costs the user eight
presses in the rare case and costs the model nothing.

## Notes for whoever builds it

`token.create` already accepts a client-minted `tokenId`, checked by `isCanonicalUUID` — it was
added so that undoing a *deletion* could put a token back under the id the room still knew it by.
Creating N tokens with N minted ids reuses that path exactly, and the undo entries are the
`deleteToken` variant already on `HistoryAction` read the other way round.

Watch the render cost: N creates means N broadcasts, and `renderTokens` in `game-canvas.svelte`
rebuilds every token group on change. Twenty in quick succession may need coalescing. That's a
thing to measure, not a reason to design differently.

Not asked for yet, and deliberately out of scope: a GM toggle to switch Player token creation off
(a natural fourth item under `Manage room` — see
[full-bleed-map-layout](full-bleed-map-layout.md)), and any per-Player cap on how many tokens
can exist at once. Nothing stops a Player making twenty tokens twenty times.

## Related user stories

- [player-create-own-tokens](../user-stories/player-create-own-tokens.md)
- [room-member-create-several-tokens-at-once](../user-stories/room-member-create-several-tokens-at-once.md)

## What shipped

All of the shape above. `token.create` lost its `requireGM` and gained the per-field split
`token.update` already used: a non-GM's owner is forced to the creator and the visibility to
`visible`, both **ignored rather than rejected**. `token.delete` became "a GM, or the owner", with
the hidden-token refusal worded as a missing token even for its own owner — the same sentence, for
the same reason, as the update handler's.

**`tokenId` became `tokenIds`, and that's the decision to understand before touching this again.**
The count has to be a *server* field or the cap is unenforceable, but the client also has to know
the ids up front, because `token.created` carries no sender and one undo entry per token is
otherwise impossible to attribute. So the client mints N ids, the server places and names them,
and the ids come back in order. `RoomClient` records each entry **on the echo**, not on the send:
the name (`Monkey 3`) and the square are the server's to decide, and an undo entry holds the whole
token. A refusal therefore leaves nothing on the stack, for free.

`spawnCells` in `internal/ws/spawn.go` does the placement, by Chebyshev ring, testing whole
footprints rather than corners so a 2×2 token doesn't get a monkey dropped under its bottom-right
square. **A count of one returns the origin unconditionally**, occupied or not — without that
carve-out, undoing a deletion would politely restore the token *beside* its own square whenever
anything had parked there since, and that's the whole point of restoring it by id.

Not done, and now more visible than before: a GM switch to turn Player token creation off, and any
cap on how many tokens one Player can have standing at once. Twenty at a time, as often as they
like. Both were out of scope above and both stay out; see
[full-bleed-map-layout](full-bleed-map-layout.md) for where the switch would live.

Two things found on the way, neither caused by this work:

- **The new-token dialog was one field away from being taller than the window**, and a dialog
  taller than the window puts its own close button off the top of the screen — it is centred by
  translating half its height, so there is nothing to scroll. Adding a full-width `How many` row
  crossed that line and `asset-library.spec.ts` caught it as an unclickable `Close`. Fixed by
  putting the count on the name's row rather than by making dialogs scroll: `max-h` +
  `overflow-y-auto` on the shared `Dialog.Content` fixes the symptom everywhere and introduces a
  scroll position that clicks and fills then race against, which is a worse trade in a suite that
  drives dialogs constantly. **The taller `TokenDetailDialog` still has the original problem on a
  short viewport** — that's the one worth fixing properly, and it wants its own item.
- **Re-adding identical image bytes can clear an asset's credit on Windows.** The library JSON
  comes back with `attribution: ""` while the row that produced it had one, and the server log
  shows `rename … Access is denied` from the blobstore plus a `UNIQUE constraint failed:
  asset.content_hash` — so the upload takes the dedup path and the room-asset update writes an
  empty credit over a real one. `asset-library.spec.ts:188` is where it surfaces, intermittently.
  Unrelated to tokens; recorded here because the reproduction is easy to lose.
