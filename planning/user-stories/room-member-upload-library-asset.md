---
title: Room Member uploads an asset to the library
created: 2026-07-29
---

As a Room Member
I want to upload a new image to my room's asset library with optional attribution/license info
So that it can be reused across scenes and tokens in this room, and properly credited

## Acceptance criteria

- [ ] Upload accepts a single optional free-text attribution/license field
- [ ] Uploading a file identical to one already in the library reuses the existing stored file instead of duplicating storage, even if it was first uploaded in a different room
- [ ] The uploaded asset is immediately available in this room's library for anyone in the room to pick
- [ ] The asset does not appear in other rooms' libraries unless separately uploaded there
