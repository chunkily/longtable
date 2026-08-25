---
title: Host configures the server from the command line
created: 2026-08-25
status: incomplete
---

As a Host who works in a terminal
I want every setting readable and writable from the command line, and my whole configuration in
one file I can keep
So that running headless, scripting a deployment or checking my config into git are all still
first-class

## Acceptance criteria

- [ ] Every setting the admin UI can change, I can change from the command line
- [ ] I can list every setting with its value, its default, and what it does — without opening a
      browser or reading Go source
- [ ] Reading one value gives me the bare value, so it is useful inside a script
- [ ] A mistyped setting name is refused and the near miss named, rather than accepted and ignored
- [ ] A value the server can't use is refused when I set it, not at the next start
- [ ] I can export my whole configuration to one file and replay it on another machine
- [ ] All of it works against a server that is currently running

## Notes

This is the half of [ADR-0009](../decisions/0009-admin-ui-and-database-config.md) that keeps the
technical Host whole. Dropping the assumption that every Host is technical is not the same as
assuming none of them are, and a design that made the easy path easy by making the expert path
unpleasant would have failed.

The fourth criterion is inherited rather than new. The TOML parser refused an unknown key and drew
the line it was on, and its own comment calls that "the one failure a config file invents that a
command line never had". Deleting the file must not delete the guarantee.

The sixth is what a config file was quietly providing: one artefact to read, diff, version and
copy. Setting values one command at a time does not replace that on its own.
