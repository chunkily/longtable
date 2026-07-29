---
title: Host configures asset size limits
created: 2026-07-29
---

As a Host
I want to configure a default per-asset size limit and a total storage limit in my server's config file
So that I can prevent uploads from exhausting my server's storage

## Acceptance criteria

- [ ] Per-asset max size is a config file setting, with a sensible built-in default if unset
- [ ] Total storage size limit is a config file setting, with a sensible built-in default if unset
- [ ] An upload exceeding the per-asset limit is rejected with a clear error
- [ ] An upload that would push total storage past the configured limit is rejected with a clear error
- [ ] No web admin UI is required; changing limits means editing the config file and restarting the server (see host-config-file)
