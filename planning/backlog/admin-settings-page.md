---
title: The admin settings page
created: 2026-08-25
status: open
tags: [host, ui, admin]
story: host-configures-server-in-browser
---

The screen a Host configures the server from, and the reason
[ADR-0009](../decisions/0009-admin-ui-and-database-config.md) exists. Needs
[settings-in-database](settings-in-database.md) for the registry and
[admin-surface](admin-surface.md) for somewhere authenticated to put it.

**It renders the registry rather than restating it.** Every field, its help text, its default and
its validation come from the same table the CLI reads, so a setting cannot exist in one and not
the other. ADR-0009's rule is that a setting reachable only by clicking is a bug, and generating
the form from the registry is what makes that structural instead of a thing to remember.

- [ ] A page listing every setting with its current value, its default, and the one-line summary
      that replaced the config file's comments
- [ ] Saving validates through the registry's `Parse` — the same code path `config set` uses, so
      the two cannot disagree about what a valid value is
- [ ] A bad value is refused **at the field**, while the person can still see what they typed.
      This is the main thing the database buys over the file, where a typo instead stopped the
      server at next boot
- [ ] Settings that need a restart say so, next to the setting, rather than a save implying a
      change that hasn't happened. `addr` is one by nature; most will be at first, because live
      reload is per-setting plumbing rather than something the table grants
- [ ] The token is exchanged for a session on arrival and the query string dropped, so it doesn't
      sit in history or leak in a referrer

Not settled: whether this is a route in the existing SvelteKit app or a small separate page.
Serving it from the main frontend means the admin bundle ships to every Player too, and it has to
be reachable on a *different port* from the game — which the SPA's routing and the embed both have
opinions about. Worth deciding before wiring anything, the way
[options-page](options-page.md) asked for a mock-up first.

Note [options-page](options-page.md) is `dropped` and does **not** cover this: it was about a Room
Member's device-level theme control, a different role and a different kind of setting. It does not
argue against a Host-facing settings screen.

## Related user stories

- [host-configures-server-in-browser](../user-stories/host-configures-server-in-browser.md)
