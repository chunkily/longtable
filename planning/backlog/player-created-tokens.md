---
title: Player-created tokens, and creating several at once
created: 2026-08-04
status: open
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
