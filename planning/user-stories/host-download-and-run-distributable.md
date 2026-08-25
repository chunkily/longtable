---
title: Host downloads and runs a prebuilt distributable
created: 2026-07-29
status: incomplete
---

As a Host
I want to download a prebuilt distributable and run it directly
So that I can self-host the server without installing a build toolchain or compiling anything myself

## Acceptance criteria

- [ ] Prebuilt distributables are published for common platforms/architectures (e.g. Linux, macOS, Windows on amd64/arm64)
- [ ] Running the server requires nothing beyond the OS itself — no separate runtime, compiler, or system library installation
- [ ] Starting the server from a fresh download is a single command with no build step

## Still incomplete, on the first criterion only

The second and third hold and were checked rather than assumed: a cross-built binary was run on a
machine with no Go toolchain, no Node and nothing else installed, and it served the embedded
frontend, the API and the SPA fallback, creating its own database and asset directory beside
itself. That is the whole of "nothing beyond the OS", and it is what
[ADR-0005](../decisions/0005-webp-reencoding-library.md) bought by keeping cgo out of the build.

The first says distributables *are published*, and none are. The machinery that publishes them
shipped with [automated-release-ci](../backlog/automated-release-ci.md) and fires on a `v*` tag;
what is left is cutting the first one, which settles what version this project is at and is the
GM's call rather than an unbuilt piece of work. This flips to `done` with that tag, as does
[distributable-releases](../backlog/distributable-releases.md).
