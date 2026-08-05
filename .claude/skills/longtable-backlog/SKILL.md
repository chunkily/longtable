---
name: longtable-backlog
description: How Longtable's planning/ directory works — the backlog (one flat folder, with an `open`/`done` status field in frontmatter), user stories with acceptance criteria and their own status field, and ADRs. Use this whenever a task involves picking work up from the backlog ("what's open?", "what's next?"), finishing an item and recording what shipped, checking whether a user story is actually done, adding a new backlog item or user story, or writing an architecture decision record. Also read it before starting any feature in this repo, since the backlog item and its linked user story are where the acceptance criteria and the already-settled design decisions live — starting from the code alone means re-deciding things that were deliberately decided.
---

# The planning directory

`planning/` is the spec. Read the backlog item *and* its linked user stories before writing code:
they carry the acceptance criteria, and often a note recording something already ruled in or out
(fog is manual-only for now; the eraser's sweep counts as one undo per deletion; grid alignment
moved to upload time). Re-deciding those by accident is the main failure mode here.

```
planning/
├── roles.md              Role glossary — Host, GM, Player, Room Member, Visitor, Developer
├── backlog/              one file per item, `status:` in frontmatter — open, done or dropped
├── user-stories/         one story per file, "As a … I want … So that …" + acceptance criteria
└── decisions/            ADRs, `NNNN-slug.md`
```

**Status lives in frontmatter, not the folder — for both backlog items and user stories.**
Backlog items are `status: open` or `status: done`, stories are `status: incomplete` or
`status: done` (see [User stories](#user-stories)). Either can also be `status: dropped` —
decided against, kept for the reasoning. Changing status means editing that field in
place, never moving the file — nothing that links to an item goes stale when it ships. This used
to be a three-folder (`open/`/`in-progress/`/`done/`) scheme; it flattened because every item made
two folder hops over its life, and anything that had linked to it while `in-progress` broke again
the moment it shipped. There's no `in-progress` state any more — items are meant to be small
enough to land in one session, so `open` covers "not started" and "not finished yet" both.

A backlog item reaching `status: done` doesn't automatically mean its story is `done` too — a
story can need more than one backlog item, or an item can ship less than its story asked for.

## Dropping something

`status: dropped` is for work decided against. **Don't delete the file** — write a
`## Why this was dropped` section under the original text and leave the acceptance criteria or
sub-tasks exactly as they were. What's being preserved is the reasoning, and it only makes sense
next to the thing it's reasoning about. A dropped item that names what replaced it, and why the
replacement is a better fit, is the single most useful thing to find when the same idea comes back
six months later wearing a different name — which is the usual reason anyone reads `planning/` at
all.

`gm-set-room-visibility.md` is the worked example: public/private rooms, dropped because the
premise (an audience of strangers who can reach the server) doesn't exist in a self-hosted LAN
product. Deleting it would have left nothing to stop the next person proposing it.

Keep the original slug when dropping, even if it now names something that doesn't exist —
`visitor-browse-public-rooms.md` still spells a role that was retired with it, and that's a
feature of the record rather than an untidiness to fix.

## Picking something up

`backlog/` is one flat folder, so finding the queue is a grep:

```bash
grep -l '^status: open' planning/backlog/*.md
```

That also matches `README.md`, whose format template sits at column 0 inside a code fence — one
hit to ignore, not an extra item.

Prefer an item that is genuinely self-contained, and say why you chose it. If one item unblocks
another, the foundation is usually the better pick: the distance measuring tool shipped the
ephemeral-broadcast plumbing that the area-of-effect tool needs.

`open` covers both "not started" and "started, not finished". There is deliberately no third
state — items are meant to be small enough to land in one session, so the case barely comes up.
The cost is that frontmatter alone won't tell you whether someone has already been in here, and
other sessions do work in this same checkout. When that matters,
`git log -- planning/backlog/<slug>.md` and the item's own prose are the only signals there are.

Items name the files and line numbers they concern, and that's often the fastest orientation
available — but it goes stale. Verify before relying on it.

## Item format

```markdown
---
title: Short title
created: YYYY-MM-DD
status: open
tags: [optional, labels]
story: optional-user-story-slug
---

What this is, and any context or sub-tasks (`- [ ]` checkboxes).

## Related user stories

- [slug](../user-stories/slug.md)
```

`story:` in the frontmatter is for a single story; a "## Related user stories" list is for
several. Both link into `user-stories/`. New items go in `backlog/` with a kebab-case slug and
`status: open`.

One trap when writing any frontmatter in this repo, including this skill's own: a colon followed
by a space ends a key in YAML, so prose *about* the status field — spelling it `status: open`
inline in a title or description — silently breaks the whole block. It cost this skill its
description once; the loader fell back to the body's first heading and nothing announced the
failure. Reword to "an `open`/`done` status field", or quote the value.

## Finishing an item

1. Flip the item's frontmatter to `status: done`.
2. Add a **`## What shipped`** section to the file. Keep the original description above it
   untouched — the point is to read what was asked for and what actually happened side by side.
3. Check the linked user story's acceptance criteria against what actually shipped — don't check
   off individual `- [ ]` boxes, those stay as a checklist, not a tracker. If every criterion is
   now true, flip the story's frontmatter to `status: done`. If only some are, leave it
   `status: incomplete` and say why in the story itself (a line naming which criterion is still
   open, and what it's blocked on if anything) — the same instinct as a "What shipped" note, just
   living in the story instead of the item. Verify against the code, not against the backlog
   item's own status: notes go stale, and "the item is `status: done`" is not the same claim as
   "every criterion in the story is true today."
4. If the work laid groundwork for another item, add a short note to *that* item pointing at what
   now exists. That's how the next session finds it.
5. Update "Where things stand" in `CLAUDE.md` — a shipped feature moves out of the gap list, and
   a gap you closed shouldn't still be advertised. Anything the change made untrue in the skill
   references goes in the same commit; the `longtable-feature` skill lists what to check.

All of that belongs in the commit that ships the work, not a tidying pass afterwards. This repo
is early enough that a doc a week behind is worse than no doc — someone trusts it and loses an
hour.

The "What shipped" note is written for whoever touches the area next, not as a changelog. Look at
`eraser-tool.md` and `measuring-tool-distance.md` for the shape of it: what a user can
now do, then the two or three decisions that would be expensive to rediscover — the constraint
behind an odd-looking choice, a trap that only shows up with two clients, a caveat that's known
and accepted. Skip anything `git log` already answers.

## User stories

One file per story, kebab-case slug prefixed with the role (`gm-`, `player-`, `room-member-`,
`host-`, `developer-`). Roles come from `roles.md` — use `room-member-` when a story applies to
both GMs and players, and don't invent a role that isn't in that file. One was invented once, in
passing, while writing an unrelated story; it survived only because a feature was written to suit
it, and both had to be removed together. `roles.md` says so at the foot.

```markdown
---
title: Short title
created: YYYY-MM-DD
status: incomplete
---

As a <role>
I want <goal>
So that <benefit>

## Acceptance criteria

- [ ] ...
```

New stories are always `status: incomplete`. Flip it to `status: done` only once you've verified
every acceptance criterion against the actual code — not assumed from a linked backlog item's
own status. If a criterion has gone stale (a later, deliberate design change replaced part of what it
asked for, rather than nobody having built it yet), don't force `done` to make it go away: leave
it `incomplete` and add a short note under the acceptance criteria explaining what changed and
why, then either rewrite the criterion to match or leave it as a marker that the story needs a
rewrite. See `room-member-select-token.md` for the shape of that note (names an unbuilt
dependency). `gm-pick-map-from-library.md`'s history is the other case: its criteria assumed
inline upload, a later deliberate design change replaced that with a link-out flow, and once the
criteria were rewritten to match the new design it went to `status: done` — don't leave a story
stuck `incomplete` forever just because its criteria, not the code, turned out to be wrong.

Acceptance criteria are the spec for the feature and are worth reading literally — "visible to all
Room Members in real time while I'm actively measuring" and "disappears once I finish; it isn't
persisted" between them settled the entire wire design for measuring. When a criterion is
ambiguous, say which reading you took and why.

## Decisions

`decisions/NNNN-slug.md`, numbered in order (0001–0008 so far: self-hosted multi-room, Go backend,
SQLite, Svelte+Konva, WebP re-encoding, config file format, the table is trusted, seats and
sessions). Write one when choosing between technologies or committing to an architecture, not for
ordinary implementation choices — those belong in the code comment next to the thing they explain,
which is this codebase's habit.

The last two are worth reading before proposing anything about permissions or identity.
[0007](../../../planning/decisions/0007-the-table-is-trusted.md) is the reason Longtable keeps
declining to stop Room Members doing things they could misuse: role boundaries (GM vs Player) are
enforced, identity boundaries (person vs person) are not. A suggestion to add a per-seat password,
a claim approval or a per-token lock is arguing against it, which is allowed but should be done
knowingly rather than as an oversight being tidied up.
