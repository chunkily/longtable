---
title: Record a hash of every uploaded file as it arrived
created: 2026-10-06
status: done
tags: [assets, data-model, moderation]
story: host-moderate-assets
---

An asset's `content_hash` is of the **re-encoded output**, deliberately (`storeImage` in
`internal/api/assets.go` says why: a PNG and a JPEG of the same picture should share one stored
WebP). That is the right key for dedup and the wrong one for blocking. Anything that changes the
output changes the hash, and [frame-token-art-at-upload](frame-token-art-at-upload.md) makes every
upload of the same file come out differently — a Player re-uploading a removed portrait will frame
it a few pixels off without even trying to, and a block keyed on the output hash lets it straight
through. The map grid aligner already has the small version of this: the same map aligned twice is
two hashes.

So record a **source hash** too — SHA-256 of the request's file bytes, before decoding, offset or
crop — so that [track-removed-asset-hashes](track-removed-asset-hashes.md) can block the file
however it was framed.

- [ ] Every upload records its source hash against the asset it produced
- [ ] One asset can have several sources (the PNG/JPEG case) and one source several assets (two
      crops), so this is its own table — `asset_source (asset_id, source_hash, …)` or similar —
      rather than a column on `asset`
- [ ] A re-add of something the server already holds still records the new source against it
- [ ] Removing an asset from a room's library leaves the record alone, as it leaves the `asset` row

**Ship this with the cropper or ahead of it, not with the moderation work.** A source hash cannot be
recovered afterwards — the original bytes are thrown away at re-encode — so everything uploaded
before this exists can only ever be blocked by its output hash. Every week it waits is a week of
uploads that can't be fully moderated. On its own it is a table and a few lines in the upload path.

Known and accepted: an *edited* source (resized, recompressed, screenshotted) has a new hash and is
not caught. That wants perceptual hashing, which is out of scope — under
[ADR-0007](../decisions/0007-the-table-is-trusted.md) the case worth designing for is a careless or
stubborn table, not an adversary, and exact matching on source plus output covers it.

## What shipped

Every upload's file is hashed as it arrived — SHA-256 of the multipart file, read before the
re-encoder sees it and then rewound — and recorded against the asset it produced in a new
`asset_source (asset_id, source_hash, first_seen_at)` table, with an index on the hash.
`store.RecordAssetSource`, `AssetSources` and `AssetsFromSource` are the three queries; nothing in
the product reads them yet.

- **Cascades with the asset.** Whatever removes an asset for good has to copy its sources into a
  block list *first* — noted on the table and on
  [track-removed-asset-hashes](track-removed-asset-hashes.md).
- **A failure to record is logged, not refused.** The image is stored and usable; a missing row
  only makes a later block easier to walk past.
- `TestUploadAsset_RecordsTheSourceBehindEveryAsset` makes two assets from one file with a grid
  offset — the same shape of thing a crop will produce — and checks they share one source, that a
  repeat upload doesn't add a second row, and that the hash is of the bytes as sent. It fails with
  the recording disabled.

Shipped ahead of [frame-token-art-at-upload](frame-token-art-at-upload.md), as this item asked:
uploads from before it can only ever be blocked by their output hash.
