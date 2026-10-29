---
title: Get a Host past the operating system's first-run warnings
created: 2026-10-29
status: open
tags: [host, release, docs, setup]
story: host-download-and-run-distributable
---

The release binaries are unsigned, and each desktop OS has something to say about that, or about a
program that starts listening on the network. None of it is in `docs/hosting.md`, and every one of
them lands in the first five minutes, before a Host has seen anything of Longtable itself. Wanted
for v1.0.

**Windows Firewall** is the one that matters most, because it fails silently. The first time
Longtable listens on `:8080`, Windows asks whether to allow it, with checkboxes for private and
public networks. Cancel it, or tick only "public" while the home network is marked private, and the
server runs perfectly for the Host on `localhost` while no player can reach it — and nothing on
either screen says why. This is the likeliest "my players can't join" there is.

- [ ] Hosting guide: what the prompt looks like, which box to tick, and how to fix it afterwards if
      it was dismissed (Windows Security → Firewall → Allow an app)
- [ ] Worth a look: whether the startup output can say anything useful. Longtable can't see the
      firewall's decision, but it could print, on Windows, one line pointing at the guide beside
      the LAN addresses. Weigh that against startup output that's already two kinds of address

**Windows SmartScreen** shows "Windows protected your PC" for an unsigned `.exe` downloaded from
the internet. The way through is `More info` → `Run anyway`, which reads as exactly what a cautious
person shouldn't click.

- [ ] Hosting guide: say it will appear, why (the binary isn't code-signed), and the two clicks

**macOS Gatekeeper** refuses an unsigned, un-notarised binary outright ("cannot be opened because
the developer cannot be verified"). Right-click → Open once, or System Settings → Privacy &
Security → Open Anyway, or `xattr -d com.apple.quarantine longtable` for a terminal user. A
downloaded binary may also have lost its executable bit depending on how it was unpacked.

- [ ] Hosting guide: the right-click route first, the `xattr` line for terminal users
- [ ] Check the macOS archive keeps the executable bit when unpacked by Finder

**Linux** mostly has nothing to warn about, except that a desktop file manager may not run a
binary on double-click at all, and some distros' firewalls (`ufw`, `firewalld`) block `:8080` by
default.

- [ ] One line each, in the same section

## Signing, later

Code signing and notarisation make SmartScreen and Gatekeeper go away, and cost money (an
Authenticode certificate; an Apple Developer membership) plus secrets in the release workflow.
Not for v1.0 — the docs route is free and covers it — but worth revisiting if the project picks up
Hosts who aren't comfortable clicking through. The firewall prompt appears whether or not the
binary is signed.

## Related user stories

- [host-download-and-run-distributable](../user-stories/host-download-and-run-distributable.md)
