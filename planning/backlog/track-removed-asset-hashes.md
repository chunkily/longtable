---
title: Track removed asset content hashes
created: 2026-07-29
status: open
tags: [assets, data-model, moderation]
story: host-moderate-assets
---

Support blocking re-uploads of content a Host has removed. Today, dedup only checks the live
`asset` table via `FindAssetByHash` — if a removed asset's row is simply deleted, its content
hash goes with it, and a byte-identical re-upload would look like a brand-new file and get
accepted again.

Note that a *room* can now take an asset off its own library — see
[remove-asset-from-room-library](remove-asset-from-room-library.md). That deletes a
`room_asset` row and nothing else: the `asset` row, its content hash and the blob all survive, so
it lays no groundwork here and must not be mistaken for it. This item is still about a Host
deleting the file itself, which is where the hash goes missing.

Needs a record that survives asset removal (e.g. a separate `blocked_content_hash` table, or a
"removed" flag/tombstone on the asset row instead of a hard delete) containing at least: the
content hash, removal timestamp, and the Host's optional reason. The upload path needs to check
this before accepting a new file.

**The output hash alone is not enough to block on** (2026-10-06). `content_hash` is of the
re-encoded WebP, and [frame-token-art-at-upload](frame-token-art-at-upload.md) means the same file
cropped two ways is two hashes — a Player re-uploading a removed portrait will frame it differently
without trying to, and walk straight past a block keyed on the output. The grid aligner already has
the smaller version of this. Removing an asset should therefore block its output hash **and** every
source hash [record-upload-source-hash](record-upload-source-hash.md) recorded against it, and an
upload is refused if either matches. Assets uploaded before source hashes were recorded can only be
blocked by output; that's a permanent gap for them, not something to backfill. An edited source
(resized, recompressed) is not caught either, knowingly — see that item's last paragraph.
