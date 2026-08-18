---
title: Host-configurable asset limits
created: 2026-07-29
status: open
tags: [assets, host]
story: host-configure-asset-limits
---

Let a Host configure per-asset and total storage size limits in the server's config file, with
sensible built-in defaults, so uploads can't exhaust server storage. Depends on
[host-config-file](host-config-file.md) existing first.

**That now exists** (2026-08-18): `internal/config` holds the settings struct, the defaults and
the commented template the server writes for a Host, and `docs/hosting.md` has the table of
settings. Adding limits means a field in each of those three places, plus a row in that table —
and this is the item that earns TOML's `[section]` grouping, since upload limits are the first
settings that don't belong beside `addr` and `database`. Note the ordering rule in `Config`'s doc
comment: every top-level key has to stay above the first `[section]` header in the generated file.
