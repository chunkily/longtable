---
title: A failed start takes its own error message with it
created: 2026-08-25
status: open
tags: [host, cli, setup]
---

`docs/hosting.md` opens by telling a Host to run Longtable by double-clicking it. When that
works, the console stays up and shows the addresses to hand out. When it *fails*, the console
closes with the process — so a Host whose server won't start sees a black window appear and
vanish, and never reads the error.

Which is a shame, because the errors are good. The config parser draws the offending line with
the key underlined; `-config` at a path that isn't there says so; a busy port produces a clear
bind failure. All of that is written for someone who can read it, and the one launch method the
documentation recommends is the one that throws it away.

Found while designing [ADR-0009](../decisions/0009-admin-ui-and-database-config.md)'s first-run
setup, but it is not blocked on any of it and does not depend on where settings live — a bad
`-db` path or an occupied port fails the same way afterwards.

- [ ] On a fatal startup error, when stdin is a terminal, wait for Enter before exiting so the
      message can be read
- [ ] Only when interactive — a service, a container or the e2e harness must still exit
      immediately, and `go-isatty` is the right test rather than `os.ModeCharDevice`, which
      counts `NUL` as a terminal (see
      [first-run-terminal-wizard](first-run-terminal-wizard.md) for the measurement)
- [ ] Say what it is waiting for. A bare pause with no prompt reads as a hang

Worth considering together with the exit path generally: `main` already prints fatal errors to
stderr rather than through `slog`, deliberately, because the config parser's multi-line output
survives that and doesn't survive being quoted onto one line. Whatever waits for Enter belongs
next to that decision.
