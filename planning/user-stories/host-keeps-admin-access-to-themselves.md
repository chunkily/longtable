---
title: Host keeps admin access to themselves
created: 2026-08-25
status: incomplete
---

As a Host
I want the server's settings reachable only by me, not by everyone at the table
So that handing someone a room code doesn't hand them my server

## Acceptance criteria

- [ ] Someone on the same network who knows the server's address cannot reach the admin screen at
      all
- [ ] Reaching it still needs a secret, even from the machine the server runs on
- [ ] I can find that secret again after the fact, and replace it if I want to
- [ ] Putting a reverse proxy in front of the server does not quietly open admin access to
      everyone
- [ ] The documentation says plainly what this does and does not protect against

## Notes

[ADR-0007](../decisions/0007-the-table-is-trusted.md) trusts the table, and this is deliberately
outside it: role boundaries inside a room are one thing, and the server's own configuration is
not a room matter. `roles.md` is explicit that a Host runs the server and needn't be at any table.

The fourth criterion is a specific trap rather than a general wish. The cheap implementation —
one listener, with admin routes rejecting a non-loopback client address — passes every other
criterion here and fails this one completely: behind nginx every request appears to come from
127.0.0.1. A listener actually bound to loopback cannot fail that way.

The fifth is there because the honest answer is not "this is secure". There is no TLS, so a token
crosses the network in the clear; what makes that acceptable is that the admin listener isn't on
the network at all. A Host who changes that has given the protection up, and should be told so
rather than discovering it.
