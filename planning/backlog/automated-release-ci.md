---
title: Automated release CI pipeline
created: 2026-07-29
status: done
tags: [developer, release]
story: developer-automated-release-ci
---

Automate release builds via GitHub Actions: a version tag triggers a workflow that builds the
frontend, embeds it into the Go binary, cross-compiles for all target platforms, and publishes
the artifacts to a GitHub Release, gated on existing CI checks passing.

## What shipped

`.github/workflows/release.yml`, on `push: tags: ['v*']`. Six targets — Linux, macOS and Windows
on amd64 and arm64 — built from one Linux runner.

**The gate calls `ci.yml` rather than restating it.** `ci.yml` gained a `workflow_call:` trigger
and the release has a `ci:` job that `uses: ./.github/workflows/ci.yml`, with the build job behind
`needs: ci`. Copying the four checks into a second file would have meant two definitions of "CI
passes" that drift the first time one is added to; this way a release waits for whatever CI means
that day. Only the build job takes `contents: write`, so the gate runs on a read-only token.

**One job looping over `GOOS`/`GOARCH`, not a matrix of six runners.** This is the workflow
[ADR-0005](../decisions/0005-webp-reencoding-library.md) was choosing for when it took a
WASM-embedded libwebp over cgo bindings: `CGO_ENABLED=0` cross-compiles all six from one box.
Verified — all six build clean, ~20-22MB each, and a cross-built Windows binary was run and
served the embedded frontend, the API and the SPA fallback with nothing installed. A matrix would
also have built the frontend six times, where building it once is what makes every distributable
carry the same one by construction rather than by six builds happening to agree.

Decisions worth not re-making:

- **`gh release create`, not a third-party release action.** It is already on the runner, and
  everything else here is `actions/*`. A release action is a lot of supply chain for an upload.
- **tar for Unix, zip for Windows.** A release asset is just bytes, so a zipped Unix binary
  arrives without permission to run.
- **The archive unpacks into a versioned folder**, not flat. The server writes `longtable.toml`,
  its database and its asset directory into whatever directory it is run from, so a flat tarball
  puts a Host's rooms wherever they happened to extract it.
- **The executable bit is set explicitly and then read back out of the finished tarball**, failing
  the release if it isn't there. `go build` already produces 0755 on Linux, so the `chmod` looks
  redundant — but a mode is invisible in a release listing, so the way anyone finds out it was
  missing is "permission denied", once, on a download they just made.

**One thing was not verified and could not be**, which is the mode assertion passing. Git Bash on
NTFS treats `chmod` as a no-op and WSL here has no Go toolchain, so the good case needs the Linux
runner. What *was* proved locally is that the check fires: run on Windows, the real script text
stops with `::error::…carries a non-executable longtable`. So the first tag either produces a
correct tarball or a red run — not a quiet bad download.

The rest was verified by extracting the `run:` block straight out of the YAML and executing it
with a `zip` shim, so the thing tested is the text that ships rather than a paraphrase of it.
