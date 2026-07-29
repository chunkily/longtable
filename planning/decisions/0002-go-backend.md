# ADR-0002: Go for the backend

**Status:** Accepted
**Date:** 2026-07-27 (documented retroactively on 2026-07-29 — see note below)
**Deciders:** Developer

> Like [ADR-0001](0001-self-hosted-multi-room.md), this decision predates this repo's planning
> process and is documented retroactively. The reasons confirmed by the Developer when asked are
> single static binary distribution (for Go over Node/Python) and code accessibility to
> unfamiliar contributors (for Go over Rust) — other plausible factors (concurrency model, prior
> language familiarity) were **not** confirmed as part of the actual decision, so they're
> deliberately left out of this record rather than assumed.

## Context

Given the self-hosted model decided in [ADR-0001](0001-self-hosted-multi-room.md), a Host needs
to be able to run their own instance with as little friction as possible — see
[host-download-and-run-distributable](../user-stories/host-download-and-run-distributable.md).
The backend language needed to support that: a Host should be able to download one file and run
it, without installing a separate language runtime first.

## Decision

Use Go for the backend.

## Options Considered

### Option A: Node.js/TypeScript

**Cons:** Running the server normally requires the Node runtime to be installed on the Host's
machine. Producing a true self-contained single-file binary needs extra bundling tooling
(e.g. `pkg`/`nexe`) layered on top of the normal Node workflow, rather than being the default
build output.

### Option B: Python

**Cons:** Same shape of problem as Node — normally requires a Python interpreter on the Host's
machine. Standalone-binary packaging (e.g. PyInstaller) exists but isn't the default, idiomatic
way to ship a Python program.

### Option C: Rust

**Cons:** Rust also produces self-contained static binaries, so it would have satisfied the
single-binary distribution requirement just as well as Go. It was set aside on a different
dimension: the Developer's belief that Rust code is harder for an unfamiliar developer to read
and write than Go — a real concern for a project that may want outside contributors, given Rust's
steeper learning curve (ownership/borrowing, lifetimes) compared to Go's deliberately small
language surface.

### Option D: Go — CHOSEN

**Pros:** `go build` produces a single statically-linked binary by default — no separate runtime
needs to be installed on the Host's machine, and (per the cross-compilation discussion in
[ADR-0005](0005-webp-reencoding-library.md)) it can even be cross-compiled for other platforms
from one build machine without extra tooling, as long as no cgo dependency is introduced. Also
judged more approachable than Rust for an unfamiliar contributor to pick up.

## Trade-off Analysis

Against Node and Python, single-binary distribution was the deciding factor, and Go's toolchain
produces that by default in a way neither does without extra packaging steps. Against Rust — which
would have satisfied the binary-distribution requirement just as well — the deciding factor was
different: code accessibility. Go's smaller language surface was judged easier for an unfamiliar
developer to read and write than Rust's, which matters for a project that may draw on outside
contributors (the same underlying concern named for the framework choice in
[ADR-0004](0004-svelte-konva-frontend.md), where React's larger contributor pool was weighed
against Svelte's technical fit).

## Consequences

- The single-binary distribution goal this decision serves directly shaped
  [ADR-0005](0005-webp-reencoding-library.md)'s WASM-embedded-over-cgo choice: a cgo dependency
  would have undermined the same simple-build, single-binary property Go was chosen for here.
- Frontend assets are embedded into the Go binary via `go:embed` (see [assets.go](../../assets.go)),
  keeping the "one file, no separate assets to ship" property intact end to end.

## Action Items

None — already implemented at initial commit. No further action from this ADR.
