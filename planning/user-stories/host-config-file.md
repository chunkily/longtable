---
title: Host configures the server via a config file
created: 2026-07-29
status: incomplete
---

As a Host
I want the server to persist its settings in a config file, created automatically with sensible defaults if none exists
So that I can configure my server by editing a plain file rather than managing environment variables, and get a working setup out of the box with no manual setup step

## Acceptance criteria

- [ ] On startup, the server looks for a config file at a well-known location
- [ ] If the file doesn't exist, the server creates one populated with sensible defaults, then continues starting up using those defaults
- [ ] The file is TOML, so Hosts can add comments explaining what each setting does
- [ ] All server settings are configured through this file; there's no environment variable equivalent
- [ ] A malformed config file fails startup with a clear error, rather than silently falling back to defaults
