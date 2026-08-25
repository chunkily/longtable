---
title: Change the listen address without dropping the server
created: 2026-08-26
status: open
tags: [host, admin, reliability]
story: host-finishes-setup-and-starts-playing
---

`addr` needs a restart to take effect, and every other setting says so honestly because
`Setting.Live` is false across the registry. That is fine for a running table and a real problem
for the end of a setup wizard, which wants to hand the Host straight to their own server at
whatever address they just chose.

**The answer is not to restart the process.** A double-clicked binary has no supervisor: if it
exits, nothing brings it back, and the Host is left looking at a browser tab that will never load
with no window to read an error in. Re-executing itself is worse — on Windows the console the
process owns goes with it.

Rebind in place instead. `http.Server` can be handed a listener and shut down without the program
ending, so changing `addr` is a new `net.Listen` and a `Shutdown` of the old one.

## The rule that makes it safe

**Bind the new address before releasing the old one**, and if the new bind fails, keep the old and
report the error:

```go
listener, err := net.Listen("tcp", next)   // fails? nothing has been given up yet
if err != nil {
    return err                             // still serving on the old address
}
go newServer.Serve(listener)               // accepting before anything is torn down
old.Shutdown(ctx)
```

Ordered the other way — shut down, then bind — a port that is already taken leaves the Host with
**neither** address: no game, no admin page, and no way to fix it except the CLI. That is the
failure this item exists to make impossible, and it is one line's worth of ordering.

It also means the redirect at the end of the wizard is safe by construction rather than by timing:
by the time anything responds to the Host, the new listener is already accepting.

- [ ] Hold the game listener in something that can be replaced, rather than a bare
      `http.ListenAndServe`
- [ ] Bind-before-release, with the old listener kept on failure
- [ ] Report a failed rebind to whoever asked for it, in the form they asked from
- [ ] `Setting.Live` becomes true for `addr` — the first setting it is true for, and the point at
      which the field stops being decoration

## Two wrinkles worth knowing before starting

- **Same port, different host.** Moving `:8080` to `127.0.0.1:8080` cannot bind first on Linux,
  which refuses the overlapping bind — and *can* on Windows, which allows it and hands loopback to
  the more specific listener (see [admin-surface](admin-surface.md), where that behaviour bit a
  Host). A same-port change may have to be shutdown-then-bind, accepting a brief window, or be
  refused as needing a restart. Decide deliberately rather than discovering it.
- **Everything else still needs a restart.** `assets` is held by the blobstore and
  `departure_grace` by the hub, both built at startup. This item is about `addr` alone, because
  `addr` is the one the wizard's handoff depends on. Making the others live is separate work and
  should not be smuggled in here.

## Related user stories

- [host-finishes-setup-and-starts-playing](../user-stories/host-finishes-setup-and-starts-playing.md)
