---
title: Host finishes setup and starts playing
created: 2026-08-26
status: incomplete
---

As a Host who has just answered the setup questions
I want to end up on a working Longtable, at the address I chose
So that setting it up and using it are one continuous thing rather than two

## Acceptance criteria

- [ ] Finishing setup puts me on Longtable itself, without my having to read, copy or type an
      address
- [ ] The settings I chose are in effect when I get there, including the address I am on
- [ ] I never land on a connection error, a "site can't be reached", or a 404
- [ ] If a setting I chose can't be applied — a port already in use — I am told, on the page I was
      on, with the server still running and still reachable
- [ ] Refreshing where I land doesn't put me back in the wizard

## Notes

The third criterion is the whole story, and the other four exist to make it true rather than
likely. Two of them are about the same failure from opposite ends: the server has to still be
listening somewhere at the moment the browser is sent anywhere, and the address it is sent to has
to be one the browser can actually reach.

"Following a restart" is the obvious way to read this and is the way it must not be built. A
double-clicked binary has no supervisor: a process that exits to apply a setting is a process that
does not come back, and the Host is left watching a tab load forever with no window left to show
them an error. The listener is rebound in place instead — see
[rebind-without-dropping-the-server](../backlog/rebind-without-dropping-the-server.md).

The fourth is what stops the third being achieved by never changing anything. A wizard that
refuses to apply a port because applying it might fail has kept the promise and lost the point.
