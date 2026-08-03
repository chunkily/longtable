---
title: Never generate a room slug that spells something offensive
created: 2026-08-03
tags: [rooms, safety]
---

Room slugs are six random characters out of an alphabet that includes vowels, so a fresh room
could come back as `fuck7a` or `2fagxy`. The slug is the one string everyone at the table reads
aloud and pastes into a group chat, so a GM has no graceful way to deal with a bad draw — there's
no rename, and the fix is to create another room and re-send the link. Filter the generator
instead.

## What shipped

Slug generation moved out of `internal/store/room.go` into `internal/store/slug.go`, which now
generates and re-rolls until the slug clears a blocklist, capped at 100 attempts. `CreateRoom`'s
own retry loop is unchanged and still only handles collisions — a rejected slug never reaches it.

The parts worth knowing before editing the list:

- **The existing alphabet does most of the work.** `slugAlphabet` has no `i`, `l` or `o` (dropped
  for legibility, not for this), and no digit reads as one of them, so no word
  spelled with any of those three is reachable at all. That's most of the words you'd expect on a
  list like this — `shit`, `bitch`, `cock`, `slut`, and the worst of the racial slurs, all
  impossible before the filter runs. The list only has to cover what's left.
- **Digits are de-leeted before matching**, so `a55hat` and `f4gzzz` are caught by the `ass` and
  `fag` entries rather than needing spellings of their own. The mapping over-matches on purpose
  (`9` → `g`); a false rejection costs one more turn of the loop.
- **Matching is on substrings, and entries stay off the list unless they're serious.** Both are
  the same trade in different directions: a slug is a meaningless string, so nothing innocent is
  worth defending, but every entry raises the rejection rate for every room ever created and a
  room called `turd4x` is a joke, not an incident.
- `TestSlugBlocklist_EntriesAreReachable` fails on any entry longer than a slug or containing a
  letter the alphabet can't spell, so the list can't quietly fill up with entries that look like
  protection and match nothing. If `slugAlphabet` or `slugLength` ever changes, that test is what
  tells you the list needs another pass — a shorter slug would strand the six-letter entries, and
  restoring `i`/`l`/`o` would reopen everything the alphabet is currently blocking for free.

Room *names* are user-typed and unfiltered; this is only about the generated slug.
