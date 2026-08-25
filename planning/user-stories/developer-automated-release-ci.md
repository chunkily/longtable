---
title: Developer automates release builds via GitHub CI
created: 2026-07-29
status: done
---

As a Developer
I want releases built and published automatically via GitHub Actions
So that cutting a release doesn't require manually building and uploading binaries for every platform myself

## Acceptance criteria

- [ ] Pushing a version tag (e.g. `v1.2.3`) triggers a release workflow
- [ ] The workflow builds the frontend and embeds it into the Go binary, then cross-compiles distributables for all platforms/architectures Hosts need (Linux/macOS/Windows on amd64/arm64)
- [ ] Built artifacts are attached to a GitHub Release automatically, with no manual upload step
- [ ] The release only proceeds if the existing CI checks (build, vet, test, lint, e2e) pass

## Verified

All four, in `.github/workflows/release.yml` — see
[automated-release-ci](../backlog/automated-release-ci.md) for the decisions behind it.

The fourth is worth reading carefully, because there are two ways to satisfy it and only one of
them stays true. The release doesn't repeat the checks; it calls `ci.yml`, which gained a
`workflow_call:` trigger for the purpose. A copy would have passed this criterion on the day it
was written and quietly stopped meaning it the first time a check was added to CI and not to the
copy.

**No tag has been pushed, so the workflow has never run on GitHub.** What backs this off is
local: the six cross-compiles were run and their binaries executed, and the packaging step was
tested by extracting the `run:` block from the YAML and executing that text. The one link that
needs the real runner is the executable-bit assertion, and it is written to fail the release
rather than pass quietly, so the first tag can produce a red run but not a bad download.
