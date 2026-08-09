---
title: Print the addresses players can connect to, on startup
created: 2026-08-09
status: done
tags: [host, dx]
story: host-startup-prints-lan-url
---

Nothing happens at a Longtable table until somebody reads out an address: the Host runs the
server and everyone else joins over the LAN. Finding that address means going hunting through OS
network settings for your own IP, which is the very first thing anyone has to do and the least
related to playing D&D.

Print the LAN-reachable URLs at startup, beside the listening address. No configuration — the
machine already knows.

## Related user stories

- [host-startup-prints-lan-url](../user-stories/host-startup-prints-lan-url.md)

## What shipped

`internal/lanurl` plus four lines of wiring in `serve`. A Host now gets, under the listening line:

```
INFO longtable: reachable at url=http://192.168.10.234:8123 interface=Ethernet
INFO longtable: reachable at url=http://172.17.224.1:8123 interface="vEthernet (Default Switch)"
```

**A specific `-addr` is answered with itself.** This is the one that would cost a table twenty
minutes: a server started on `127.0.0.1:8080` is reachable there and nowhere else, and printing
the machine's Wi-Fi address beside it would be a confident lie. Only a wildcard bind — `:8080`,
`0.0.0.0:8080`, `[::]:8080` — means "every interface", and only then is enumerating them right.

**Every candidate is printed, and none is chosen.** A machine with Wi-Fi, Ethernet and a VPN has
three, the Host is the only one who knows which network their players are on, and guessing wrong
is hardest to debug on exactly the setups where guessing is hardest. Private addresses sort first,
so a home LAN's `192.168.x.x` leads; the interface name rides along, which is what makes a
`vEthernet (WSL)` line identifiable as the one to ignore.

**IPv4 only, deliberately.** This address gets read aloud or typed with a thumb, and an IPv6
address is neither. Loopback is skipped because it works for exactly the person who doesn't need
it, and link-local (`169.254.x.x`) because it means DHCP failed — a symptom, not somewhere to send
a table.

`For` takes the interface list rather than reading it, which is the whole reason the rules are
testable: the interesting machines are ones nobody has to hand — three interfaces, a VPN, a laptop
with the cable out and Wi-Fi off. `Interfaces()` reads the real ones, and a failure there is a
warning rather than a failed start: the server is already up, and the worst case is a Host looking
their address up the way they always have.

Verified against this machine, on both paths: a wildcard bind printed the Ethernet address first
and the two Hyper-V switches after it; `-addr 127.0.0.1:8124` printed only itself.
