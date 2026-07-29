---
title: Host identifies and removes offending assets
created: 2026-07-29
---

As a Host
I want to identify and remove offending assets on my server
So that I can moderate content without building a full administration interface

## Acceptance criteria

- [ ] I can list uploaded assets with identifying metadata (room, filename, size, upload time)
- [ ] I can remove a specific asset by its ID
- [ ] Removing an asset deletes the underlying file and stops it from being served
- [ ] Removing an asset that's in use repoints any scene or token referencing it to a built-in default placeholder image (a small red X), rather than leaving a dangling ID
- [ ] Removing an asset blocks future re-uploads of the same content (by content hash) from being stored again
- [ ] A blocked re-upload is rejected with a clear error, not silently accepted or silently dropped
- [ ] When removing an asset, I can optionally record a reason
- [ ] Room Members reuploading a blocked asset can see the reason, if one was given, and are given a default response otherwise.
- [ ] This is accomplished via a CLI command or direct database/filesystem access, not a web admin UI
