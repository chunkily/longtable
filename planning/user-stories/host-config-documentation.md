---
title: Host reads config file documentation
created: 2026-07-29
status: incomplete
---

As a Host
I want documentation explaining the server's config file and all its settings
So that I can configure my server without reading the Go source code

## Acceptance criteria

- [ ] `docs/hosting.md` documents the config file's location and every available setting, including what each one controls and its default
- [ ] This documentation lives under `docs/`, separate from the README, so the README stays focused on development/architecture rather than growing a Host-facing reference section
- [ ] The documentation is updated whenever a new Host-configurable setting is added
