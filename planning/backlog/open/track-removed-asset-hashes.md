---
title: Track removed asset content hashes
created: 2026-07-29
tags: [assets, data-model, moderation]
story: host-moderate-assets
---

Support blocking re-uploads of content a Host has removed. Today, dedup only checks the live
`asset` table via `FindAssetByHash` — if a removed asset's row is simply deleted, its content
hash goes with it, and a byte-identical re-upload would look like a brand-new file and get
accepted again.

Needs a record that survives asset removal (e.g. a separate `blocked_content_hash` table, or a
"removed" flag/tombstone on the asset row instead of a hard delete) containing at least: the
content hash, removal timestamp, and the Host's optional reason. The upload path needs to check
this before accepting a new file.
