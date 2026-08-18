---
title: Document the host config file
created: 2026-07-29
status: done
tags: [host, docs]
story: host-config-documentation
---

Write the actual `docs/hosting.md` content once [host-config-file](host-config-file.md) exists —
every setting, its default, and what it controls. `docs/hosting.md` currently exists only as a
stub.

## What shipped

`docs/hosting.md` gained a **Settings** section: where the file is, that the server writes it
itself, a table of every setting with its default and what it does, how to run a second server
with `-config`, and what happens to a typo. The stub paragraph promising this is gone.

Two things it says that are easy to get wrong later:

- The generated file carries the same explanations in comments, so **there are two places a Host
  learns a setting exists** and both have to move when one is added. That is now a trigger in
  `CLAUDE.md`'s doc-currency list, which is the mechanism this story's third criterion asks for.
- The recovery commands (`room list`, `room reset-password`) grew a line saying to run them where
  `longtable.toml` is, or to pass `-config`. They read the database path from that file now, so a
  Host running them in the wrong directory would be told the room doesn't exist.
