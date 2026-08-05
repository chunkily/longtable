# Backlog

One markdown file per item, all in this one folder. Status is a frontmatter field, not a folder:

- `status: open` — not started (or not finished — see below)
- `status: done` — shipped
- `status: dropped` — decided against; the file stays, with a `## Why this was dropped` section
  saying what changed the mind and what replaced it

There's no `in-progress` state. Items are meant to be small enough to land in one session; if one
turns out not to be, it stays `open` rather than getting a third state to sit in.

To change status, edit the field in place — no file move, so nothing that links to the item
(other backlog items, user stories, `CLAUDE.md`, skill references) goes stale when it ships. Kept
this way on purpose: a prior three-folder version of this (`open/`/`in-progress/`/`done/`) meant
every item made two hops over its life, and anything that had linked to it while it was
`in-progress` broke again the moment it shipped.

## Format

```markdown
---
title: Short title
created: YYYY-MM-DD
status: open
tags: [optional, labels]
story: optional-user-story-slug
---

Description of the item. Add notes, sub-tasks (as `- [ ]` checkboxes), or
context as needed.
```

`story` is optional and points to a slug in [../user-stories](../user-stories) for the "why" behind the item.

## Adding an item

Create a new file here named with a short kebab-case slug, e.g. `backlog/eraser-tool.md`, with
`status: open`.
