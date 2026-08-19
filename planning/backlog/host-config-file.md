---
title: Host config file (persisted settings, auto-created with defaults)
created: 2026-07-29
status: done
tags: [host, data-model]
story: host-config-file
---

**One setting is already waiting for this.** `-banner` shipped as a flag
([host-banner-message](host-banner-message.md)), which means the Host's message is fixed for the
life of the process. It belongs in this file, and a file that can be re-read is what would let a
Host change the message without restarting everyone's sockets.

Add a TOML config file as the single source of truth for server settings — chosen over JSON so
Hosts can comment their config, and over YAML to avoid its indentation/type-coercion footguns.
See [ADR-0006](../decisions/0006-config-file-format.md). On startup, the server looks for it
and creates one with sensible defaults if it's missing. Replaces environment-variable
configuration entirely — every other Host-configurable setting (e.g.
[host-asset-limits](host-asset-limits.md)) should read from this file rather than the
environment.

## What shipped

`longtable.toml` in the working directory, written with defaults and a comment per setting the
first time the server runs, holding `addr`, `database`, `assets`, `banner` and `departure_grace`.
`internal/config` is the package; `pelletier/go-toml/v2` is the dependency ADR-0006 accepted, for
its strict mode.

**Every setting flag is gone**, from `serve` and from the `room` CLI both. `-config` is the only
flag left anywhere, and the room commands find the database through the same file the server
does — two ways of naming one database is how a Host ends up resetting a password in a file the
running server has never opened, and being told the room doesn't exist. Nothing was left behind to
tell a Host that `-db` moved: the GM was clear that nothing is released yet and preferred the
cleaner CLI to a migration path.

The decisions worth not re-making:

- **An unknown key stops the server**, naming it and drawing the line it's on. That is the one
  failure a config file invents that a command line never had — the Host edits, restarts, and
  watches the setting do nothing. An *absent* key takes its default silently, which is what lets a
  later version add a setting without breaking every file already out there. The two rules only
  work together.
- **`-config` at a path that isn't there is refused**, while a missing file at the default
  location is created. Someone who types a path has a file in mind, and writing a fresh default
  one there would start a server that ignores everything they configured and says nothing about
  it.
- **The file is a template, not marshalled output.** No marshaller keeps comments, and comments
  are the whole reason ADR-0006 chose TOML. It follows that the file is written once and never
  rewritten: a Host's own notes and ordering are theirs.
- **Fatal errors print to stderr rather than through `slog`.** The parser answers a typo by
  drawing the offending line with the key underlined, and slog quotes that onto one line as a
  string full of escaped newlines — which is the opposite of the clear error the story asks for.
- **Flat keys for now.** Five settings in one group don't earn a `[section]`;
  [host-asset-limits](host-asset-limits.md) will, and that item now carries the note about TOML
  requiring top-level keys above the first table header.

Not done: **re-reading the file while the server is up**, which is what this item's own first
paragraph wanted for the banner. Out of scope on the GM's call, and it isn't free — the trigger
has to be chosen (a signal, or watching the file) and half these settings can't change under a
running server anyway: `addr` is bound and `database` is open. `banner` and `departure_grace` are
the two that could, which is the shape a future item should start from.

## Update, 2026-08-19 — banner moved back out

This item's own opening paragraph argued for absorbing `-banner` on the reasoning that a
re-readable file is what would let a Host edit the message without restarting. That reload never
got built — the note directly above says so — and it turned out to be the wrong fix for the wrong
layer anyway: `longtable set-banner`/`clear-banner` now write straight to the database instead,
which needs no reload logic because the server was never holding a stale copy to begin with — see
[host-banner-message](host-banner-message.md).

`longtable.toml` now holds `addr`, `database`, `assets` and `departure_grace`. Everything else
above — the strict unknown-key error, the once-written template, `-config`'s two behaviours —
is unchanged and still describes the file as it stands.
