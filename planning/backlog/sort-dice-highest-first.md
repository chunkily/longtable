---
title: Show each roll's dice highest first
created: 2026-10-06
status: done
tags: [chat, dice, playtest]
story: room-member-counts-a-big-roll
---

Playtest feedback: counting successes in `/roll 12d6` means reading twelve numbers in the order they
fell. Sort them.

`internal/dice` turns each `NdM` term into text as it rolls it — `4d6(3,1,6,2)+2` — and that string
is what `message.roll_breakdown` stores and chat shows. So the sort belongs in Go, before
`joinInts`: the stored message is then already sorted, for everyone, after a refresh and for a late
arrival. Sorting in the client would mean parsing numbers back out of a formatted string.

- [ ] Within each term, dice are listed highest first — successes against a target cluster at the
      front, so counting stops at the first miss
- [ ] Each term on its own: `1d20+3d6` sorts the three d6s and leaves the d20 alone, since mixing
      dice of different sizes into one list means nothing
- [ ] A test rolls something large (`100d6`) and asserts the breakdown is non-increasing per term
- [ ] Rolls already in the log are left as they were

**Roll order is dropped, on purpose.** Nothing at a D&D table cares which die landed first, and the
total is unchanged. Recorded here so nobody later "fixes" its loss.

Considered and declined: the roller counting successes itself (`/roll 10d6>=5` → "4 successes").
Sorting was judged enough.

## What shipped

Every `NdM` term's dice are listed highest first — `/roll 4d6+2` reads `4d6(6,5,3,1)+2` — for
everyone, since the sort happens in `internal/dice` before the breakdown becomes the text that
`message.roll_breakdown` stores. Rolls already in a room's log keep the order they were rolled in.

`slices.Sort` then `slices.Reverse` on the term's own slice, *after* it has been summed, so the total
can't be touched by it. `TestRoll_ListsEachTermsDiceHighestFirst` rolls `40d6+1d20-30d6+3` fifty
times and checks three things: each term is non-increasing, each term still has its count of dice,
and the dice shown sum back to the total — that last one is what proves the sorted list is the dice
that were actually rolled. Confirmed by sorting ascending instead and watching it fail.
