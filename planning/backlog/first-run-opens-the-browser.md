---
title: First run opens the Host's browser at the admin URL
created: 2026-08-25
status: open
tags: [host, setup, admin]
story: host-sets-up-a-new-server
---

Double-click the binary, and the thing you configure it with is simply there. This is the moment
the non-technical path turns on, and it is the difference between a console line a Host has to
notice, understand and copy, and a browser that is already open on the right page. Syncthing does
exactly this and is the nearest analogue there is — self-hosted, LAN, GUI on loopback.

Needs [admin-surface](admin-surface.md), since there has to be a URL worth opening.

**The launch is three lines of `os/exec`** — `rundll32 url.dll,FileProtocolHandler` on Windows,
`open` on macOS, `xdg-open` elsewhere — started and never waited on. Less code than justifying a
dependency for it. Note `rundll32` rather than `cmd /c start`, which treats the first quoted
argument as a window title and swallows the URL.

Four guards, and the third is not optional:

- [ ] **Cannot delay or fail the start.** Fired after the listener is up, in a goroutine, error
      dropped. No default browser, or an `xdg-open` that sits there, must cost nothing — the
      console line stays the real answer and this is the convenience on top of it
- [ ] **First run only**, keyed on an explicit `setup.completed` setting rather than the database
      looking empty. A server that opened a browser on every restart is one people learn to dread
- [ ] **A flag suppresses it**, because `web/e2e/run-app.mjs` wipes its database on every run and
      is therefore *always* a first run — 174 specs would each launch a browser. Containers,
      services and anyone scripting a deployment need the same switch
- [ ] **Skipped where there is no desktop.** No `DISPLAY`/`WAYLAND_DISPLAY` on Linux means a NAS
      or a Pi; a Windows service has no interactive session to open into

Startup output ends up saying two different things, which is worth keeping distinct — the URL you
hand out, and the one that is yours:

```
Longtable is running.

  Players join at   http://192.168.1.23:8080
  Set up at         http://127.0.0.1:8081/admin?token=k3f9…

Opening your browser…
```

## Related user stories

- [host-sets-up-a-new-server](../user-stories/host-sets-up-a-new-server.md)
