---
title: Host sets up a new server
created: 2026-08-25
status: incomplete
---

As a Host
I want a new server to walk me into configuring it, without my having to find or edit a file
So that I can get a table running without knowing what TOML is

## Acceptance criteria

- [ ] Running the binary for the first time gets me to a configuration screen without my having
      to read, copy or type anything
- [ ] On a machine with a desktop, that screen opens in my browser by itself
- [ ] If it can't — no desktop, no default browser, a machine I reached over SSH — the server
      still starts, and still tells me where to go
- [ ] I can reach a working server by answering nothing at all: every default is a valid choice
- [ ] It happens once. Later starts don't reopen anything
- [ ] Nothing about this blocks a server that is started by a script, a service or a container

## Notes

The first criterion is the one that carries the story. A console line naming a URL is not the
same as arriving at the URL: a Host who doesn't know what a port is has to notice the line, know
it is meant for them, and get it into a browser correctly, and every one of those is somewhere to
lose them.

The fifth is what keeps this from becoming a nuisance for the Host who already knows what they
are doing, and the sixth is not hypothetical — the e2e harness wipes its database on every run,
so it is a first-time setup 174 times a run.
