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

`status` is `incomplete` or `done` — flip it to `done` only once every acceptance criterion is
verified true of the current code, not assumed from a linked backlog item's status. This is
separate from backlog item status, which is tracked by folder (see
[../backlog/README.md](../backlog/README.md)).

Backlog items can reference a story by its slug via a `story` field in their frontmatter (see [../backlog/README.md](../backlog/README.md)).
