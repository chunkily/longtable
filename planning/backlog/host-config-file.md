---
title: Host config file (persisted settings, auto-created with defaults)
created: 2026-07-29
status: open
tags: [host, data-model]
story: host-config-file
---

Add a TOML config file as the single source of truth for server settings — chosen over JSON so
Hosts can comment their config, and over YAML to avoid its indentation/type-coercion footguns.
See [ADR-0006](../decisions/0006-config-file-format.md). On startup, the server looks for it
and creates one with sensible defaults if it's missing. Replaces environment-variable
configuration entirely — every other Host-configurable setting (e.g.
[host-asset-limits](host-asset-limits.md)) should read from this file rather than the
environment.
