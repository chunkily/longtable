---
title: Host configures the server via a config file
created: 2026-07-29
status: done
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

## Verified, and one criterion that was aimed at something else

Four of the five hold as written: the server looks for `longtable.toml`, writes a commented one
full of defaults when it finds nothing and carries on with those defaults, the file is TOML, and a
malformed one stops startup with the offending line drawn out rather than falling back silently.

**"There's no environment variable equivalent" was aiming at the wrong target.** Longtable never
read an environment variable; what it had was five flags on `serve` and a `-db` on each `room`
subcommand. That is what this criterion was really about, and it is what shipped: every setting
flag is gone and `-config` is the only flag left, so the file is genuinely the one place settings
live. The criterion is left as written rather than rewritten, because the *intent* — one source of
truth, not two — is what was built.
