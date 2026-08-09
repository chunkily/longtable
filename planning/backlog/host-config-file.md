---
title: Host config file (persisted settings, auto-created with defaults)
created: 2026-07-29
status: open
tags: [host, data-model]
story: host-config-file
---

**One setting is already waiting for this.** `-banner` shipped as a flag
([host-banner-message](host-banner-message.md)), which means the Host's message is fixed for the
life of the process. It belongs in this file, and a file that can be re-read is what would let a
Host change the message without restarting everyone's sockets.

Add a TOML config file as the single source of truth for server settings — chosen over JSON so
Hosts can comment their config, and over YAML to avoid its indentation/type-coercion footguns.
See [ADR-0006](../decisions/0006-config-file-format.md). On startup, the server looks for it
and creates one with sensible defaults if it's missing. Replaces environment-variable
configuration entirely — every other Host-configurable setting (e.g.
[host-asset-limits](host-asset-limits.md)) should read from this file rather than the
environment.
