# ADR-0003: SQLite for storage

**Status:** Accepted
**Date:** 2026-07-27 (documented retroactively on 2026-07-29 — see note below)
**Deciders:** Developer

> Like [ADR-0001](0001-self-hosted-multi-room.md) and [ADR-0002](0002-go-backend.md), this
> decision predates this repo's planning process and is documented retroactively. When asked, the
> Developer confirmed SQLite was the default choice for being low-cost and low-overhead for
> Hosts, with no specific alternative (e.g. Postgres/MySQL, another embedded store) seriously
> weighed against it. This ADR is accordingly narrower than most others in this set — it explains
> why the default was the right call given [ADR-0001](0001-self-hosted-multi-room.md)'s
> constraints, rather than recording a real comparison process.

## Context

Given the self-hosted model decided in [ADR-0001](0001-self-hosted-multi-room.md), a Host runs
their own instance with as little operational overhead as possible — the README states this
outright: "No external services, no separate database to install." Whatever storage engine
Longtable uses needs to fit that constraint.

## Decision

Use SQLite (via `modernc.org/sqlite`, a pure-Go driver with no cgo) as the storage engine.

## Options Considered

No alternative was actually evaluated against SQLite — it was the default choice, not the winner
of a comparison process. The reasoning below is the author's own supporting analysis for why the
default holds up, not a reconstruction of a deliberation that didn't happen.

A traditional client-server database (Postgres, MySQL) would need a Host to install, run, and
keep alive a separate database server process alongside the Longtable binary — directly against
the "no separate database to install" goal, and against the single-binary distribution property
[ADR-0002](0002-go-backend.md) chose Go for in the first place. SQLite, as an embedded
database, needs none of that: it's just a file the Go process reads and writes directly.

## Trade-off Analysis

SQLite is the only option here that doesn't require the Host to run or manage anything beyond the
Longtable binary itself, matching the low-cost, low-overhead requirement directly. The
`modernc.org/sqlite` driver specifically avoids cgo, keeping it consistent with the rest of the
project's build-simplicity decisions ([ADR-0002](0002-go-backend.md),
[ADR-0005](0005-webp-reencoding-library.md)).

## Consequences

- Longtable's data (rooms, participants, scenes, assets, etc.) lives in a single SQLite file per
  Host, with no separate database process to configure, secure, or keep running.
- SQLite's single-writer model is a reasonable fit for the scale this project targets — a handful
  of concurrent rooms per Host, not a multi-tenant service with heavy concurrent write load. This
  wasn't a factor the Developer named as part of the decision, but is worth naming as a limit: if
  Longtable's usage pattern ever changes significantly (e.g. very high concurrent write volume
  across many simultaneous rooms), this is the assumption that would need revisiting.

## Action Items

None — already implemented at initial commit. No further action from this ADR.
