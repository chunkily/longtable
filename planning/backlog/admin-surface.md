---
title: The admin surface — a loopback listener and a token
created: 2026-08-25
status: done
tags: [host, security, admin]
story: host-keeps-admin-access-to-themselves
---

The authenticated place the admin UI will live, built before there is a UI to put in it. Needs
[settings-in-database](settings-in-database.md) first, since the token and `admin_addr` are stored
settings.

[ADR-0009](../decisions/0009-admin-ui-and-database-config.md) records why this is a gate rather
than a formality: [ADR-0007](../decisions/0007-the-table-is-trusted.md) trusts the *table*, and
server configuration is not a table matter — a Host runs the server and need not be at any table.

**Two listeners, not one with a check.** The game keeps `addr`; admin gets `admin_addr`,
defaulting to `127.0.0.1:8081`. The tempting alternative — one listener, and the admin routes
reject a non-loopback `RemoteAddr` — was ruled out: the moment a Host puts nginx or Caddy in
front, every request arrives *from* 127.0.0.1, the check passes for the whole internet, and
nothing looks wrong. A listener actually bound to loopback is structurally safe rather than
conditionally safe, because the OS will not route external packets to it whatever headers arrive.

**The token stays anyway**, because loopback is not a trust boundary. Any local process or user
reaches 127.0.0.1, a website the Host visits can issue blind requests to it, and DNS rebinding
points an attacker's domain at it. The two cover different holes: the token alone loses to a LAN
sniffer, loopback alone loses to local processes and the browser.

- [ ] `admin_addr` setting, default `127.0.0.1:8081`, on its own listener
- [ ] `config set admin_addr` **warns** when the value is not a loopback address rather than
      silently accepting it — the registry's `Parse` is the place for that
- [ ] Token: 32 bytes via `auth.NewToken`, which already exists and is already URL-safe. Stored,
      printed at startup as part of a URL, compared with `subtle.ConstantTimeCompare`
- [ ] The token travels in a header or body, **not** a plain auto-sent cookie — blind CSRF from a
      page the Host is visiting works otherwise. If it is a cookie it needs `SameSite=Strict` and
      an `Origin` check
- [ ] `Host` header validated against the expected loopback address, which is the DNS-rebinding
      defence Jupyter shipped after being bitten by its absence
- [ ] `longtable admin` — open or `--print` the admin URL, and rotate the token. This is also the
      answer to a Host who lost the startup line, which on Windows happens whenever the console
      closes with the process
- [ ] The plaintext caveat written down in `docs/hosting.md`: there is no TLS, so the token
      crosses the LAN in the clear exactly as Jupyter's does. Loopback is what makes that
      acceptable, and a Host who moves `admin_addr` off loopback has given that up

**Deliberately not built: a remote-admin opt-in.** Loopback plus a token plus a CLI that can do
everything covers every case anyone has constructed, and `ssh -L 8081:localhost:8081` gives a
remote Host the UI *with* encryption. Adding an exposure later is easy; walking one back after
Hosts depend on it is not.

## What shipped

`internal/admin`, on its own listener at `admin_addr` (`127.0.0.1:8081`), with a placeholder page
behind the guard so the surface could be tested as a surface before there was anything on it.
`longtable admin` prints the URL, `-new` rotates.

**Verified against a running binary rather than argued for.** With `addr = :8080`, `netstat` shows
`0.0.0.0:8080` and `127.0.0.1:8081` — and the game answered on the machine's LAN address while the
admin port refused the connection outright. That is the property the whole item exists for, and it
is a fact about the socket rather than about any code path.

Three defences, each covering what the others don't, and all tested:

- **The loopback binding**, which is the one that survives a reverse proxy.
- **The `Host` header check** — DNS rebinding. A page the Host is merely visiting can point a name
  it controls at 127.0.0.1; what it cannot do is forge the header the browser sends. A valid token
  with a foreign `Host` gets a 404, and `localhost` / `127.0.0.1` / `[::1]` are all accepted
  because they are all the same machine.
- **A `SameSite=Strict`, `HttpOnly` cookie**, exchanged for the token and redirected so the secret
  leaves the URL — history and referrers included, which is the correction Jupyter made after
  shipping the naive version.

**One real bug, found by driving it rather than by reading it.** The token was held from startup,
so `longtable admin -new` printed "any link you shared before now stops working" while the old
link kept working and the *new* one was refused with a 401 — exactly backwards for a Host rotating
because they leaked it. The token is now read per request, the way the banner already worked, and
there is a test pinning it. A read per request costs nothing on a surface that is one person
clicking.

**The admin page can be turned off**, with an empty `admin_addr` rather than a toggle beside it —
a boolean and an address are two settings that can disagree, and then something has to decide
which wins. Nothing binds, nothing is minted, and startup says so rather than leaving a silence
where a URL used to be. It costs a Host nothing, because ADR-0009 already requires the CLI to
reach every setting the page would have.

That turned up a latent bug in `Resolve`, which treated a stored empty string as an absent key and
substituted the default — so turning the admin page off would have silently left it on, which is
the exact "edit that does nothing" this whole design refuses. Absent and explicitly-empty are now
different answers.

**`addr` and `admin_addr` on one port is refused**, found by a Host running the game on 8081 for
acceptance testing and meeting the new default head-on. Worth knowing why it needed catching in
config rather than being left to the bind: Linux refuses the second bind and the server won't
start, which is at least loud, but **Windows allows a wildcard and a specific address to share a
port and hands loopback to the more specific one**. So the game answered on the LAN address while
`127.0.0.1:8081` quietly served the admin page, and nothing logged a thing. Reproduced, then
refused in `Validate` so both systems behave the same.

Two things deliberately left: `longtable admin` prints but does not open a browser, which belongs
with [first-run-opens-the-browser](first-run-opens-the-browser.md) and its four guards; and there
is still no remote-admin opt-in, per the note above.

## Related user stories

- [host-keeps-admin-access-to-themselves](../user-stories/host-keeps-admin-access-to-themselves.md)
