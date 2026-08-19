---
title: Rows ordered on a clock too coarse to tell them apart
created: 2026-08-19
status: done
tags: [testing, store]
---

Two Go tests failed intermittently on Windows — `TestListRoomAssets_NewestFirst` about three runs
in ten, `TestSystemMessage_JoiningPutsALineInTheLogForEveryone` about one in three. They looked
like two unrelated flakes and were one bug in two places.

`time.Now()` on Windows advances in steps of around a millisecond, and both queries ordered rows
by a `created_at`/`added_at` written from it. Anything the server does in one breath — two people
connecting, three files added from one file dialog — lands with **identical** timestamps, at which
point the ordering is whatever the tie-break says. The library's tie-break was `a.id`, a random
UUID, so "newest first" was decided by a coin toss; the chat log had no tie-break at all, so it
was SQLite's to choose and it chose differently between runs.

This is not only a test problem. A room adding three maps at once got them back shuffled in the
picker, and a chat log could show two arrivals in the wrong order — both on the machine the GM is
most likely to be hosting from.

## What shipped

Both now order by `rowid`, which is insertion order and owes nothing to a clock:

- `ListRoomAssets` (`internal/store/asset.go`) — `ORDER BY ra.rowid DESC`, replacing
  `added_at DESC, a.id`. `added_at` is still what the library *shows*; it just isn't what the rows
  are sorted by.
- `ListRecentMessages` (`internal/store/message.go`) — `created_at DESC, rowid DESC`. The
  timestamp stays primary here because a message's time is meaningful to a reader and a log is
  read as a timeline; rowid only settles ties.

Decisions worth not rediscovering:

- **rowid survives a removal, which is the part worth checking rather than assuming.** SQLite
  hands a new row the largest existing rowid plus one, so even when deleting the newest row frees
  the top value, the next insert still lands above everything still there. That is exactly what
  this ordering needs, and `TestListRoomAssets_NewestFirstAfterARemoval` pins it.
- **Re-adding bytes a room already has still doesn't promote the entry.** That path is an upsert:
  the `ON CONFLICT` branch deliberately doesn't restamp `added_at`, and an UPDATE doesn't move a
  rowid either. The two halves agree, and `TestListRoomAssets_ReAddingKeepsAnEntryWhereItWas` is
  what fails if they ever stop.
- **The tests assert the whole order, not just the head**, and neither sleeps between writes.
  Pacing them would have made them pass while testing something nobody does — adding several files
  in one go is exactly what the assets page does. Reversing the `ORDER BY` fails all of them on
  every run, which is the check that they have teeth.
- **`WITHOUT ROWID` would break this silently.** Neither table is declared that way, and nothing
  in the schema wants to be, but a table that later is would lose the column these queries sort on
  — the failure being a compile-clean query that returns rows in an order nobody chose.

## The sweep, done the same day

Every other query ordered on a bare `created_at` now carries the same tie-break. None of them had
been seen failing — this is the latent half of the same bug, fixed while the reasoning was to
hand:

- `ListDrawingsForScene` (`internal/store/drawing.go`) — ties decide **z-order** on the map, so
  two people drawing at once could swap which stroke is on top, differently on each of their
  screens.
- `initiativeOrder` (`internal/store/initiative.go`) — **the one most likely to bite.** Its three
  keys are initiative, then the manual nudge, then age, and a batch of monsters added in one go
  ties on all three by design. The comment there claimed age made it stable; it didn't, and that
  claim is corrected rather than deleted.
- `ListScenesForRoom` (`internal/store/scene.go`) — ties shuffle the Scenes dialog.
- `ListParticipantsForRoom`, `ListSeatsForRoom` and the GM-seat lookup
  (`internal/store/participant.go`) — the roster, the seat picker, and which row is *the* GM seat
  if a room somehow holds two. `p.rowid` goes in the seats query's `GROUP BY` as well as its
  `ORDER BY`: it adds no groups, being functionally dependent on `p.id`, but naming it is what
  makes it a column that query may sort on rather than one SQLite picks out of an arbitrary row.
- `ListRooms` (`internal/store/room.go`) — ties shuffle `longtable room list`, which is a Host
  reading a code off a screen.

`internal/store/order_test.go` holds one test per list. They **force** the tie — writing the rows,
then flattening `created_at` by hand — because a machine whose clock can separate them would
otherwise order them correctly with or without the tie-break, and the test would prove nothing on
CI while the bug was live on the GM's laptop.

**One thing worth knowing before trusting those tests**, and recorded at the head of that file
too: only the *descending* one currently fails without its tie-break. Strip them all and rerun —
`TestListRooms` fails every time, and the four ascending tests still pass, because SQLite's sorter
happens to return tied rows in scan order, which is rowid ascending, which is what they assert.
That is an accident of the query plan and not a promise: no order is specified for tied rows, and
`ListRecentMessages` — descending, same shape — was reordering itself one run in three. So the
ascending four pin intent rather than catching today's bug, and the tie-breaks are what turn
"happens to work" into "says what it means".
