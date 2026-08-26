---
title: A setup wizard on first run, with an express way through it
created: 2026-08-26
status: open
tags: [host, setup, admin, ui]
story: host-sets-up-a-new-server
---

The admin page exists and renders every setting ([admin-settings-page](admin-settings-page.md)),
but a first-time Host meeting six fields at once has been handed a form, not walked through
anything. This is the walk: a few questions, asked once, that write the answers into the settings.

Needs [admin-surface](admin-surface.md) and [admin-settings-page](admin-settings-page.md), both
done. Pairs with [first-run-terminal-wizard](first-run-terminal-wizard.md), which is the same
questions asked in a terminal for a Host with no browser — **the two must ask the same set**, and
the way to guarantee that is to drive both from `internal/config`'s registry rather than writing a
question list twice.

## Express, and the long way round

Two routes through, and the first is the one most Hosts should take:

- **Express** — one button, every default, done. No questions at all. The defaults are already
  chosen to be the right answer for someone running a game on their own machine, and a wizard that
  makes a person confirm six of them has charged them for nothing.
- **Step by step** — the questions, one at a time, for a Host who wants the port somewhere else or
  their data somewhere else.

Express is not a shortcut past a decision; it *is* the decision, and the wizard should say so
rather than hiding it behind "advanced".

**Express must never be the more permissive route.** It takes every default, and any question
whose default is "don't" stays "don't" — the update check is the first of those, and the quick way
through setup has to also be the private one. A future setting that opted a Host into something by
being defaulted on would break that quietly, so weigh it here rather than there.

## Which questions

Not every setting deserves asking about. The wizard is not a second rendering of the settings
form:

- [ ] Decide which settings a first-time Host should be asked about at all, and mark the rest as
      "change later if you need to". `uploads.max_total_storage` is a good example of one nobody
      should have to think about on day one.
- [ ] Ask whether Longtable may check for new versions — see
      [check-for-new-versions](check-for-new-versions.md). That question is *why* it can exist at
      all: it is the only thing Longtable would ever send outside the machine, and asking once is
      what makes it a choice rather than a default nobody was told about.
- [ ] Take the question text from the registry's `Summary` rather than writing it twice
- [ ] Validate through the registry's `Parse`, so the wizard cannot accept a value the settings
      page would refuse

## Knowing it has been done

- [ ] A `setup.completed` marker, so the wizard runs once. Keyed on that rather than on the
      database looking empty — a Host who deletes their rooms has not asked to be set up again,
      and [first-run-opens-the-browser](first-run-opens-the-browser.md) needs the same flag for
      the same reason
- [ ] Until it is set, the admin page is the wizard rather than the settings form; after, the
      other way round
- [ ] Reachable again afterwards for a Host who wants it, but not the default landing

## Related user stories

- [host-sets-up-a-new-server](../user-stories/host-sets-up-a-new-server.md)
- [host-finishes-setup-and-starts-playing](../user-stories/host-finishes-setup-and-starts-playing.md)
