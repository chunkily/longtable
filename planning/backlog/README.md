# Backlog

One markdown file per item. Status is tracked by which folder the file lives in:

- `open/` — not started
- `in-progress/` — actively being worked
- `done/` — shipped (kept for history; delete if you don't want to keep it around)

To change status, just move the file to the other folder (`mv backlog/open/foo.md backlog/in-progress/foo.md`).

## Format

```markdown
---
title: Short title
created: YYYY-MM-DD
tags: [optional, labels]
story: optional-user-story-slug
---

Description of the item. Add notes, sub-tasks (as `- [ ]` checkboxes), or
context as needed.
```

`story` is optional and points to a slug in [../user-stories](../user-stories) for the "why" behind the item.

## Adding an item

Create a new file in `open/` named with a short kebab-case slug, e.g. `backlog/open/eraser-tool.md`.
