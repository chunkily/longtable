---
title: Settings move into the database, and the config file goes
created: 2026-08-25
status: open
tags: [host, data-model, config]
story: host-configures-server-from-the-command-line
---

The foundation for [ADR-0009](../decisions/0009-admin-ui-and-database-config.md). Everything else
in that decision sits on this, and nothing else can start until it lands.

**This is deliberately the largest item in the set, because it cannot be made smaller.** Deleting
`longtable.toml` breaks the e2e harness in the same breath — `web/e2e/run-app.mjs` writes one to
set `departure_grace = "2s"`, for reasons its own comment explains — so the settings table, the
CLI that replaces the file and the harness change all have to arrive together or the suite is red
in between. A staged version that reads both a file and the database is a migration path, and
ADR-0009 is explicit that there is nothing to migrate.

The shape settled in discussion:

- **One registry** in Go — key, default, one-line summary, a `Parse` that validates *and*
  canonicalises, and whether the setting takes effect live or needs a restart. `config list`, the
  comments in an export, the admin UI's fields and the validation on write all read that one
  table. It is what makes ADR-0009's "the UI and the CLI are peers" a property of the code rather
  than a promise two implementations have to keep separately.
- **`Parse` canonicalises**, so `config set uploads.max_asset_size 26214400` reads back as
  `25 MiB` and `get` round-trips what `set` accepted.
- **Dotted keys** (`uploads.max_asset_size`), which is already how the value is spelled when
  written inline in TOML — the existing config tests use exactly that form.
- **`config list` is where the comments went.** ADR-0006 chose TOML for its comments and this
  deletes the file that held them; the debt has to be paid in the CLI's output and the UI's help
  text, not skipped.
- **`export` regenerates commented TOML that the server never reads.** A technical Host gets their
  diffable, version-controllable artefact back, and `import` replays it — with no two-writers
  problem and no comment round-trip problem, because the file is output rather than source.

Sub-tasks:

- [ ] `setting` table. No `CHECK` constraints — the value set is enforced in Go, per the schema
      rule, and `TestSchema_HasNoCheckConstraints` will say so if not
- [ ] The registry, carrying the defaults and validation `internal/config` already has
- [ ] `longtable config list` / `get` / `set` / `unset` / `export` / `import`
- [ ] An unknown key is refused and the near miss named — that was the TOML parser's best
      property ("the one failure a config file invents that a command line never had") and it has
      to survive the file
- [ ] `-db` and `-addr` flags; `-config` and `internal/config`'s file handling deleted
- [ ] `run-app.mjs` sets `departure_grace` through the CLI instead of writing a file
- [ ] `docs/hosting.md`'s settings table rewritten against the new commands

Open, and worth deciding while writing it: whether `banner` joins the registry and replaces
`set-banner`/`clear-banner`. It is already stored in the database and already read live, so it
fits exactly — the argument against is that it is content rather than configuration, and
`set-banner` reads better as a verb.

## Related user stories

- [host-configures-server-from-the-command-line](../user-stories/host-configures-server-from-the-command-line.md)
