---
title: Settings move into the database, and the config file goes
created: 2026-08-25
status: done
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

## What shipped

`longtable.toml` is gone, along with `internal/config`'s reader, template and `-config`. The
package is now the registry and nothing else: `Setting{Key, Default, Summary, Live, Parse}`, five
rows, and three constructors so an entry says what a setting *is* and nothing about how its type
is handled. Values live in a `setting` table holding only what somebody actually set — absent
means "take the registry's default", which is what lets a later version add a setting without
every existing database knowing about it.

`longtable config list|get|set|unset|export|import`, all on `-db`, all working against a running
server (verified: `config set` while the server was serving, and it kept serving).

The decisions worth not re-making:

- **`Parse` validates and canonicalises in one step**, so what `set` accepts is what `get` prints:
  `26214400` comes back `25 MiB`. One parser per setting rather than one for the CLI and one for
  loading, because two would eventually disagree about what a valid value is.
- **`set` validates the whole prospective config, not the one value.** Some rules are about two
  settings at once — a total-storage limit under the per-asset limit passes on its own and leaves
  a server nothing can be uploaded to.
- **`import` validates everything before writing anything**, so a file with one bad line leaves
  the server exactly as it was rather than half-changed.
- **The file's two best properties were kept deliberately.** An unknown key is refused with the
  near miss named (Levenshtein, capped at three edits — past that a suggestion is a guess), and a
  bad value is refused *as it is typed*. The second is the real prize: the same mistake used to
  surface as a server that wouldn't boot.
- **`Resolve` ignores a stored key it doesn't know**, while the write path refuses one. Refusing
  at startup would mean a version that removed a setting couldn't boot on a database that still
  remembered it.
- **`config list` pads by hand rather than using a tabwriter.** A tabwriter aligns a run of rows,
  and the description lines between settings aren't part of the table — flushing per row gave
  every setting its own column widths and no alignment at all.
- **Everything is `Live: false`.** Live reload is per-setting plumbing, and a CLI claiming a
  change took effect when it hadn't is the one lie this must not tell.

`Export` renders the registry as commented TOML that the server never reads. Getting the ordering
wrong there is silent and nasty — TOML puts every bare key above the first `[section]` header, so
a top-level setting written after one imports back as part of it — and there's a test pinning it.

**The e2e harness now configures the server the way a Host would**, running `config set` against
the database before spawning. That was the item's whole reason for being atomic, and it works:
the presence specs, which depend on `departure_grace = 2s`, pass.

**`banner` did not join the registry.** It stays `set-banner`/`clear-banner`. It would have fit —
already in the database, already live — but it is content rather than configuration, and folding
it in would have made the one genuinely live thing sit in a registry where everything else needs a
restart. Worth revisiting when something else earns `Live`.

Not done here, and deliberately: `assets` is still a setting because blobs stay on disk, and
nothing yet stops a Host pointing it somewhere silly.

## Related user stories

- [host-configures-server-from-the-command-line](../user-stories/host-configures-server-from-the-command-line.md)
