# ADR-0001: Self-hosted, multi-room architecture

**Status:** Accepted
**Date:** 2026-07-27 (documented retroactively on 2026-07-29 — see note below)
**Deciders:** Developer

> This decision predates the planning process established in this repo's `planning/` tree and was
> already implemented at initial commit. It's documented here retroactively, from the Developer's
> recollection rather than a live discussion, so it's necessarily less detailed than
> [ADR-0005](0005-webp-reencoding-library.md) or [ADR-0006](0006-config-file-format.md).
> Originally written and numbered after those two; renumbered to 0001 once written, since it's the
> foundational architecture decision they both build on and predates them chronologically.

## Context

Longtable needed a hosting/distribution model decided before almost anything else could be built:
who runs the server, where does game data live, and does one running instance serve one table or
many.

## Decision

Longtable is self-hosted — each Host runs their own instance on their own infrastructure — and a
single running instance supports multiple independent rooms, not just one campaign per install.

## Options Considered

### Option A: Hosted SaaS (single central multi-tenant service run by the Developer)

**Pros:** No install step for end users; centralized updates; lowest friction to start playing.
**Cons:** All GMs' campaign data would live on a third party's infrastructure rather than the
GM's own — directly against the privacy/data-ownership goal below. Also makes the Developer
permanently responsible for operating a shared service for everyone, not just their own games.

### Option B: Self-hosted, single room per instance

**Pros:** Simplest possible data model — no room-scoping or multi-tenancy within a single
process.
**Cons:** A Host running multiple tables/campaigns would need a separate deployment (separate
process, port, data file) per room, which wastes the effort of setting up a Host in the first
place.

### Option C: Self-hosted, multi-room per instance — CHOSEN

**Pros:** GMs' data stays on infrastructure they control, not a third party's. A single Host
install serves multiple tables/campaigns at once, getting more value out of one deployment.
**Cons:** Requires a room-scoped data model and isolation between rooms within a single process,
more upfront complexity than either single-tenant option.

## Trade-off Analysis

The two decisive factors were data ownership (a GM's campaign shouldn't have to live on someone
else's server) and getting real utility out of a single deployment (one Host, many tables) rather
than forcing a fresh install per campaign. Both point away from a hosted SaaS and away from a
single-room-per-instance model, toward self-hosted with multiple rooms per instance.

## Consequences

- Nearly everything else in this repo's data model and planning docs assumes room-scoping: rooms,
  participants, scenes, assets, and the [Host/GM/Player/Room Member role split](../roles.md) all
  exist because of this decision.
- The Host distribution story ([host-download-and-run-distributable](../user-stories/host-download-and-run-distributable.md))
  and the self-contained single-binary approach follow from "each Host runs their own instance"
  — see [ADR-0002](0002-go-backend.md) and [ADR-0003](0003-sqlite-storage.md) (embedded storage,
  no separate database server to run).
- Multi-room support means the server has to isolate state between rooms correctly (a bug that
  leaks one room's data into another is a much more serious class of issue than in a
  single-room-per-instance design).

## Action Items

None — already implemented at initial commit. No further action from this ADR.
