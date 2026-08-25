---
title: "docs/hosting.md is wrong about what to back up"
created: 2026-08-25
status: open
tags: [host, docs]
story: host-backs-up-a-server
---

`docs/hosting.md`'s settings table says of `database`:

> Where every room and everything in it is stored. This is the file to back up.

**That is not true**, and it fails in the worst direction: a Host who follows it keeps their
rooms, scenes, tokens, chat and seats, and loses every map and every piece of token art. Uploaded
images live in the `assets` directory, addressed by content hash, entirely outside the database.

Independent of everything else — this is a current bug in shipped documentation and can land on
its own, before or after any of [ADR-0009](../decisions/0009-admin-ui-and-database-config.md).

It stays true rather than being fixed by consolidation: moving blobs into SQLite was considered
during ADR-0009 and rejected, because `modernc.org/sqlite` exposes no incremental blob I/O and
every upload and download would materialise the whole file in memory. So a Longtable installation
is two things to back up, permanently, and the documentation has to say which two.

- [ ] Correct the `database` row so it stops claiming to hold everything
- [ ] Say plainly that a backup is the database file **and** the assets directory, and that
      keeping only one of them loses the other's contents
- [ ] Mention that the two must be kept together — restoring a database beside a stale asset
      directory leaves scenes pointing at art that isn't there

Worth checking at the same time whether anything else repeats the claim. `README.md`'s Quickstart
describes where Longtable keeps its state and should agree.

## Related user stories

- [host-backs-up-a-server](../user-stories/host-backs-up-a-server.md)
