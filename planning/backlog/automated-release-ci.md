---
title: Automated release CI pipeline
created: 2026-07-29
status: open
tags: [developer, release]
story: developer-automated-release-ci
---

Automate release builds via GitHub Actions: a version tag triggers a workflow that builds the
frontend, embeds it into the Go binary, cross-compiles for all target platforms, and publishes
the artifacts to a GitHub Release, gated on existing CI checks passing.
