---
title: Host hears about a new version
created: 2026-08-26
status: incomplete
---

As a Host
I want to be told when a newer Longtable is out, if I asked to be
So that I find out about fixes without having to go and look

## Acceptance criteria

- [ ] I am asked, once, during setup, whether Longtable may check
- [ ] If I don't answer — I skipped setup, or I took the express route, or nothing ran it at all —
      it doesn't check
- [ ] When I have said yes and a newer release exists, the server says so as it starts, and says
      where to get it
- [ ] When I am up to date, it says nothing
- [ ] A server with no internet starts exactly as fast, and complains about nothing
- [ ] Nobody at my table is shown any of this

## Notes

The second criterion is the one to hold on to. This is the only thing Longtable would ever send
outside the machine it runs on, and a product that deliberately makes rooms unlistable should not
begin by phoning home on a first start nobody consented to. "Express" means every default, and the
default is off — so the quick way through setup is also the private one, and that is not an
accident to be tidied up later.

The fifth is not a nicety either: a Longtable server on a hall's wifi with no route out is a
completely normal deployment, and a check that made those Hosts wait, or warned them about
something they can't fix, would be a worse feature than no check at all.

The sixth is the same line the whole admin surface draws. A GM or a Player cannot update the
server and should not be handed a message about it — `planning/roles.md` has a Host running the
server without necessarily being at any table.
