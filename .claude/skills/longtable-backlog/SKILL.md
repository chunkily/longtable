---
name: longtable-backlog
description: How Longtable's planning/ directory works — the folder-as-status backlog, user stories with acceptance criteria, and ADRs. Use this whenever a task involves picking work up from the backlog ("pick something from in-progress and work on it", "what's next?"), finishing an item and recording what shipped, adding a new backlog item or user story, or writing an architecture decision record. Also read it before starting any feature in this repo, since the backlog item and its linked user story are where the acceptance criteria and the already-settled design decisions live — starting from the code alone means re-deciding things that were deliberately decided.
---

# The planning directory

`planning/` is the spec. Read the backlog item *and* its linked user stories before writing code:
they carry the acceptance criteria, and often a note recording something already ruled in or out
(fog is manual-only for now; the eraser's sweep counts as one undo per deletion; grid alignment
moved to upload time). Re-deciding those by accident is the main failure mode here.

```
planning/
├── roles.md              Role glossary — Host, GM, Player, Room Member, Visitor, Developer
├── backlog/
│   ├── open/             not started
│   ├── in-progress/      actively being worked
│   └── done/             shipped, kept for history
├── user-stories/         one story per file, "As a … I want … So that …" + acceptance criteria
└── decisions/            ADRs, `NNNN-slug.md`
```

**Status is the folder.** Changing status means moving the file — use `git mv` so the rename is
tracked. Nothing inside the file records status, so don't add a `status:` field.

## Picking something up

Read every file in `in-progress/` before choosing — they're small, and some carry a "## Done"
section marking a feature that's partly landed with a specific remainder (undo/redo has drawings
done and token moves outstanding). Prefer an item that is genuinely self-contained, and say why
you chose it. If one item unblocks another, the foundation is usually the better pick: the
distance measuring tool shipped the ephemeral-broadcast plumbing that the area-of-effect tool
needs.

Items name the files and line numbers they concern, and that's often the fastest orientation
available — but it goes stale. Verify before relying on it.

## Item format

```markdown
---
title: Short title
created: YYYY-MM-DD
tags: [optional, labels]
story: optional-user-story-slug
---

What this is, and any context or sub-tasks (`- [ ]` checkboxes).

## Related user stories

- [slug](../../user-stories/slug.md)
```

`story:` in the frontmatter is for a single story; a "## Related user stories" list is for
several. Both link into `user-stories/`. New items go in `open/` with a kebab-case slug.

## Finishing an item

1. `git mv planning/backlog/in-progress/<slug>.md planning/backlog/done/<slug>.md`
2. Add a **`## What shipped`** section to the moved file. Keep the original description above it
   untouched — the point is to read what was asked for and what actually happened side by side.
3. Leave the user story's `- [ ]` checkboxes alone. Shipped stories keep them unchecked; the
   folder is the status, not the checkbox.
4. If the work laid groundwork for another item, add a short note to *that* item pointing at what
   now exists. That's how the next session finds it.

The "What shipped" note is written for whoever touches the area next, not as a changelog. Look at
`done/eraser-tool.md` and `done/measuring-tool-distance.md` for the shape of it: what a user can
now do, then the two or three decisions that would be expensive to rediscover — the constraint
behind an odd-looking choice, a trap that only shows up with two clients, a caveat that's known
and accepted. Skip anything `git log` already answers.

## User stories

One file per story, kebab-case slug prefixed with the role (`gm-`, `player-`, `room-member-`,
`host-`, `visitor-`, `developer-`). Roles come from `roles.md` — use `room-member-` when a story
applies to both GMs and players.

```markdown
---
title: Short title
created: YYYY-MM-DD
---

As a <role>
I want <goal>
So that <benefit>

## Acceptance criteria

- [ ] ...
```

Acceptance criteria are the spec for the feature and are worth reading literally — "visible to all
Room Members in real time while I'm actively measuring" and "disappears once I finish; it isn't
persisted" between them settled the entire wire design for measuring. When a criterion is
ambiguous, say which reading you took and why.

## Decisions

`decisions/NNNN-slug.md`, numbered in order (0001–0006 so far: self-hosted multi-room, Go backend,
SQLite, Svelte+Konva, WebP re-encoding, config file format). Write one when choosing between
technologies or committing to an architecture, not for ordinary implementation choices — those
belong in the code comment next to the thing they explain, which is this codebase's habit.
