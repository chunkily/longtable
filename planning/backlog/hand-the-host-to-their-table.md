---
title: Finish setup by landing the Host on their own table
created: 2026-08-26
status: open
tags: [host, setup, admin]
story: host-finishes-setup-and-starts-playing
---

The last step of [first-run-wizard](first-run-wizard.md): the Host presses the button and arrives
at a working Longtable, ready to make a room. Not a "setup complete" page with an address to copy,
and above all **not a dead tab** — the requirement is that this path cannot end in a connection
error or a 404.

Needs [rebind-without-dropping-the-server](rebind-without-dropping-the-server.md), which is what
makes the promise keepable rather than likely.

## Where to send them

**Build the target from the address the Host is already using, not from the setting.** They
reached the admin page at some hostname; pair that hostname with the new player port and send
them there:

| They are on | `addr` is | Send them to |
| --- | --- | --- |
| `127.0.0.1:8081` | `:8080` | `http://127.0.0.1:8080` |
| `localhost:8081` | `:8080` | `http://localhost:8080` |

Reading the target off `addr` instead produces `http://:8080` for the commonest value there is,
which is not an address a browser can be given. It also gets a Host on `localhost` sent to
`127.0.0.1`, which works but is a needless change of origin.

- [ ] Derive the host part from the request, the port from the resolved `addr`
- [ ] Handle a bare `:port` by keeping the Host's own hostname rather than inventing one
- [ ] The edge worth naming: a Host who moved `admin_addr` onto the network and reached it by LAN
      address, with `addr` on loopback, cannot be sent anywhere that works. Say so rather than
      redirecting into a refusal

## Why it can't 404, rather than probably won't

Three things in order, and each removes a way to fail:

1. **The listener is rebound before the response is written**, so by the time the browser is told
   to go anywhere, something is already accepting there. This is the whole guarantee; the rest is
   belt and braces.
2. **A failed rebind never becomes a redirect.** The old address is still serving, the wizard says
   what went wrong, and the Host is exactly where they were.
3. **A short waiting page rather than a bare 302.** It polls `/api/healthz` on the target and
   navigates when it answers, so any residual gap is a spinner rather than a browser error page.
   Worth having even though (1) should make it unnecessary — the cost is a few lines, and what it
   replaces is the single worst screen a new Host could be shown.

Be honest about the limit: **a waiting page cannot save a server that isn't running.** If the
process ever exits as part of applying settings, there is nothing left to poll and nothing to fix
it from. That is the argument for rebinding in place rather than restarting, and it belongs in
that item.

- [ ] Mark `setup.completed` before redirecting, so a refresh of the destination doesn't reopen
      the wizard
- [ ] Land on the room list, not on a "create a room" form — the Host may want to look around
      first, and the home page already offers both

## Related user stories

- [host-finishes-setup-and-starts-playing](../user-stories/host-finishes-setup-and-starts-playing.md)
