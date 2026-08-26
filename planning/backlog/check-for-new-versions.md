---
title: Tell a Host when a newer Longtable exists, if they asked to be told
created: 2026-08-26
status: open
tags: [host, setup, release]
story: host-hears-about-a-new-version
---

A Host who downloaded a binary has no way to learn that a newer one exists. Nothing on the machine
points anywhere, and there is no package manager to notice for them.

On start, ask GitHub what the latest release is and say something if it is newer than the running
binary. That is the whole feature.

## Off unless somebody said yes

**This is the first thing Longtable would ever send outside the machine it runs on**, and that is
worth treating as a decision rather than a default. Every check tells GitHub that a Longtable
server exists at some address, and a self-hosted product whose rooms are deliberately unlistable
should not begin its life by phoning home on a first start nobody has consented to yet.

So: **a question in [first-run-wizard](first-run-wizard.md), and off for anyone who never answers
it.** That covers a service, a container, the e2e harness, a Host who skipped the wizard, and a
Host who took the express route — because express means every default, and the default here is
off. Nobody is opted in by a button that says "use the defaults", and nobody should later change
that without changing this paragraph.

Draft of the question, which is where the privacy note belongs rather than buried in a doc:

> **Check for new versions?**
> Longtable will ask GitHub for the latest release each time it starts. That's the only thing it
> ever sends outside this machine.

- [ ] `check_for_updates`, a boolean, off by default
- [ ] The registry needs a `boolSetting` constructor — it has string, duration, byte size and
      address so far
- [ ] Asked by both wizard paths, from the registry, like every other question

## What it must not do

- [ ] **Never delay or fail a start.** A Longtable server with no internet at all is a normal
      deployment, not an error: run the check after the listener is up, in a goroutine, with a
      short timeout, and drop every error silently. A Host on a disconnected laptop must not see a
      warning, let alone a slower start.
- [ ] **Say nothing when there is nothing to say.** A line every start saying "you're up to date"
      is noise that teaches people to stop reading the startup output, which is where the admin
      URL lives.
- [ ] **Not touch the banner.** That is the Host's message *to players* and points the other way;
      a Player cannot update the server and should not be told to. This is a startup log line and
      nothing more, for the same reason the whole admin surface is a Host's alone — see
      `planning/roles.md`.

## Two things that come for free

- **`/releases/latest` on GitHub excludes prereleases**, so a Host on a stable build is never
  nagged about `v1.0.0-rc1`. That pairs exactly with the `--prerelease` flag `release.yml` already
  passes for a hyphenated tag, without either end knowing about the other.
- **A build from source has no version**, so the check should skip entirely rather than compare
  against nothing. A developer being told their working tree is behind `v1.2.3` is noise, and it
  is the one machine where it is certainly wrong.

## Blocked on knowing what version this is

The binary cannot tell whether it is behind, because it does not know what it is: there is no
`-version` flag and no `ldflags` stamp. That is the loose end
[distributable-releases](distributable-releases.md) already records as something to settle when
the first tag is cut, and this is its second consumer — worth doing once, for both.

## Related user stories

- [host-hears-about-a-new-version](../user-stories/host-hears-about-a-new-version.md)
