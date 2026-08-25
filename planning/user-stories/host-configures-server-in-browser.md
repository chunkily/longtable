---
title: Host configures the server in a browser
created: 2026-08-25
status: done
---

As a Host
I want to change my server's settings on a screen, with each one explained where I'm changing it
So that configuring it doesn't mean editing a file and restarting to find out whether I got it right

## Acceptance criteria

- [ ] Every setting is listed with its current value, its default, and a plain description of what
      it does
- [ ] I can change a setting and save it without restarting the server, unless that setting is one
      that genuinely needs a restart
- [ ] A setting that needs a restart says so, next to itself, before I save
- [ ] A value the server can't use is refused while I'm still looking at what I typed, with a
      message that says what a good value looks like
- [ ] Nothing I can change here is unavailable from the command line

## Notes

The fourth criterion is the point of the whole change. With a config file a mistake is discovered
at the next startup, by which time the Host is looking at a failure rather than at the field they
got wrong.

The fifth is the rule from [ADR-0009](../decisions/0009-admin-ui-and-database-config.md) stated
from the Host's side: the screen and the CLI are two views of the same settings, and a setting
that exists only on the screen has abandoned every Host running headless.

## Verified

All five, on [admin-settings-page](../backlog/admin-settings-page.md).

The second is met in the honest direction rather than the flattering one: nothing is live yet, so
every save says a restart is needed. The criterion asks for a change to take effect without one
"unless that setting genuinely needs a restart", and today they all do — `Setting.Live` is false
across the registry, and the page reports that rather than implying otherwise.

The fifth holds by construction: the page renders from the same registry the CLI reads, so a
setting cannot appear on one and not the other.
