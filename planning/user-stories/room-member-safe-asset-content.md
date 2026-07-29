---
title: Room Member is protected from malicious asset content
created: 2026-07-29
---

As a Room Member
I want every asset to be re-encoded by the server when it's uploaded
So that I'm not exposed to malicious content hidden inside an uploaded image file

## Acceptance criteria

- [ ] Every uploaded asset is decoded and re-encoded before being stored, regardless of what was uploaded
- [ ] Only PNG, JPEG, WebP, and GIF are accepted as valid input formats
- [ ] Re-encoding strips any non-pixel data (e.g. metadata, embedded scripts, extraneous file segments)
- [ ] An upload that fails to decode as a valid image is rejected rather than stored as-is
- [ ] Only the re-encoded version is ever served; the original uploaded bytes are never served directly
