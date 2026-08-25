---
title: The admin surface — a loopback listener and a token
created: 2026-08-25
status: open
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

## Related user stories

- [host-keeps-admin-access-to-themselves](../user-stories/host-keeps-admin-access-to-themselves.md)
