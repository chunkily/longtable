---
title: Host backs up a server
created: 2026-08-25
status: incomplete
---

As a Host
I want to know exactly what to copy to have a backup
So that restoring it gives me back the rooms *and* the art, not one without the other

## Acceptance criteria

- [ ] The documentation names every file and directory a complete backup needs
- [ ] It says what is lost by copying only one of them
- [ ] It says the pieces have to be restored together
- [ ] Nothing anywhere else claims a single file is enough

## Notes

Today `docs/hosting.md` says of the database: "Where every room and everything in it is stored.
This is the file to back up." A Host who follows that keeps their rooms, scenes, tokens, chat and
seats, and loses every map and every piece of token art, which live in the assets directory
outside it.

That is worth fixing on its own and does not wait on anything. It also is not going away: moving
uploaded images into SQLite was considered during
[ADR-0009](../decisions/0009-admin-ui-and-database-config.md) and rejected on the grounds that
`modernc.org/sqlite` has no incremental blob I/O, so a Longtable installation is two things to
back up permanently.

The last criterion is why this is a story rather than a one-line edit: the claim is the kind that
gets restated. `README.md` describes where Longtable keeps its state, and any future setup screen
will be tempted to say the same reassuring thing.
