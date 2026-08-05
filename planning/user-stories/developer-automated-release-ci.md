---
title: Developer automates release builds via GitHub CI
created: 2026-07-29
status: incomplete
---

As a Developer
I want releases built and published automatically via GitHub Actions
So that cutting a release doesn't require manually building and uploading binaries for every platform myself

## Acceptance criteria

- [ ] Pushing a version tag (e.g. `v1.2.3`) triggers a release workflow
- [ ] The workflow builds the frontend and embeds it into the Go binary, then cross-compiles distributables for all platforms/architectures Hosts need (Linux/macOS/Windows on amd64/arm64)
- [ ] Built artifacts are attached to a GitHub Release automatically, with no manual upload step
- [ ] The release only proceeds if the existing CI checks (build, vet, test, lint, e2e) pass
