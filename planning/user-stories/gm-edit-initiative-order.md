---
title: GM edits initiative order
created: 2026-07-29
status: done
---

As a GM
I want to edit an entry's initiative value or manually reorder entries
So that I can correct mistakes or break ties the way my table's rules require

## Acceptance criteria

- [ ] I can change an entry's initiative value, which re-sorts the tracker
- [ ] I can manually reorder entries that share the same initiative value relative to each other
- [ ] I can toggle a freestanding entry's hidden state after adding it
- [ ] Changes are visible to all Room Members in real time

## Verified 2026-08-09

All four hold. The second is worth reading exactly as written — "entries that share the same
initiative value" — because that *is* the rule the server enforces: reordering refuses to cross a
different value, since the order is the values and a list that disagreed with the numbers beside
it would be worse than no manual order at all.

Not covered by any criterion here, and so not built: renaming a freestanding entry after adding
it. The value, the hidden flag and the manual order are editable in place; a rename is a remove
and an add.
