---
title: A first-run wizard for Hosts with no browser
created: 2026-08-25
status: open
tags: [host, setup, cli]
story: host-sets-up-a-new-server
---

The headless half of first-run setup: a NAS, a Pi, a VPS over SSH — the case
[first-run-opens-the-browser](first-run-opens-the-browser.md) cannot serve. Needs
[settings-in-database](settings-in-database.md) for something to write into.

**Lower priority than it first looks, and worth starting after the browser path rather than
before.** If the admin UI ships first, the browser open *is* the first-run experience for the
Hosts this whole direction is aimed at, and the terminal prompt is a fallback for people who are
comfortable in a terminal anyway — which is exactly the audience that can run `longtable config
set` and be done. Build it when the browser path has proved out, not in parallel with it.

Two facts established while designing this, both verified rather than assumed:

- **Input is capturable when the binary is double-clicked.** The Windows build is subsystem 3
  (`WINDOWS_CUI`), Go's default without `-H=windowsgui`, so Windows allocates a console and stdin
  is attached to it. That is the same console the Host already reads the LAN address from.
- **`os.ModeCharDevice` is the wrong test for "is anyone there".** `/dev/null` and `NUL` are
  character devices, so a service started with its input from null looks interactive to that
  check. `go-isatty` gets it right — it calls `GetConsoleMode` on Windows, which `NUL` fails —
  and it is already an indirect dependency, so promoting it costs nothing new.

- [ ] Runs only when nothing is configured **and** stdin is a real terminal
- [ ] Skippable, and non-interactive-safe: a service, a container, the e2e harness and a Host
      passing flags all reach a running server without answering anything
- [ ] Defaults good enough that answering nothing is a valid choice — every question has to be
      answerable with Enter
- [ ] Writes through the same registry `config set` uses, so the wizard cannot accept a value the
      CLI would reject

Related and worth doing whether or not this is: **a console opened by double-click closes when the
process exits**, so a Host whose server refuses to start — a bad `-db` path, a port already in use
— sees a window flash and vanish, taking the error with it. That is a real bug in the "just
double-click it" story today and deserves its own item.

## Related user stories

- [host-sets-up-a-new-server](../user-stories/host-sets-up-a-new-server.md)
