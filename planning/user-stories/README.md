# User Stories

One markdown file per story, named with a short kebab-case slug, e.g. `gm-track-token-hp.md`.

## Format

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
- [ ] ...
```

`status` is `incomplete`, `done` or `dropped`. Flip to `done` only once every acceptance criterion
is verified true of the current code, not assumed from a linked backlog item's status. Use
`dropped` for a story decided against — keep the file and the criteria, and add a
`## Why this was dropped` section explaining what the premise turned out to be and what replaced
it. Backlog items carry their own `status` field the same way (see
[../backlog/README.md](../backlog/README.md)).

Backlog items can reference a story by its slug via a `story` field in their frontmatter (see [../backlog/README.md](../backlog/README.md)).
