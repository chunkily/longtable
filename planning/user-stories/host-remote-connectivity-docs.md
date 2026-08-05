---
title: Host reads remote connectivity documentation
created: 2026-07-29
status: incomplete
---

As a Host
I want documentation covering how to make my server reachable by players who aren't on my local network
So that I can choose a connectivity approach that fits my situation instead of being stuck guessing

## Acceptance criteria

- [ ] Documentation confirms the same-network case works out of the box, with no extra setup
- [ ] Documentation explains port forwarding + Dynamic DNS as the traditional option, and notes that some ISPs (CGNAT) make this impossible no matter how it's configured
- [ ] Documentation recommends at least one tunneling tool (e.g. Cloudflare Tunnel) as an easier alternative that works behind CGNAT and needs no router access, noting it routes traffic through a third-party relay
- [ ] Documentation mentions a VPN mesh option (e.g. Tailscale) as the most private alternative, noting it requires every player to install a client
