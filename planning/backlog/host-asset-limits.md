---
title: Host-configurable asset limits
created: 2026-07-29
status: done
tags: [assets, host]
story: host-configure-asset-limits
---

Let a Host configure per-asset and total storage size limits in the server's config file, with
sensible built-in defaults, so uploads can't exhaust server storage. Depends on
[host-config-file](host-config-file.md) existing first.

**That now exists** (2026-08-18): `internal/config` holds the settings struct, the defaults and
the commented template the server writes for a Host, and `docs/hosting.md` has the table of
settings. Adding limits means a field in each of those three places, plus a row in that table —
and this is the item that earns TOML's `[section]` grouping, since upload limits are the first
settings that don't belong beside `addr` and `database`. Note the ordering rule in `Config`'s doc
comment: every top-level key has to stay above the first `[section]` header in the generated file.

## What shipped

`[uploads]` in `longtable.toml`, with `max_asset_size` (default `25 MiB`, matching the ceiling
this server enforced before it was configurable) and `max_total_storage` (default `10 GiB`). Both
are a new `config.ByteSize` — the same idea as `config.Duration`: TOML has no size type, and a
bare number would leave a Host doing the arithmetic themselves to know what they'd typed. It
parses and prints through `github.com/dustin/go-humanize`, already an indirect dependency of
something else in the module graph, so this promoted it to direct rather than adding a new one.

Enforcement lives in `internal/api/assets.go`, ahead of `storeImage`: `MaxAssetBytes` bounds the
uploaded multipart body the way the old hardcoded constant did, and a new `checkTotalStorage`
checks `store.TotalAssetBytes()` — a `SUM(byte_size)` over the whole `asset` table, which is the
server-wide total since assets are content-addressed and shared across rooms — against
`MaxTotalStorage`. **That check is dedup-aware**: it looks the re-encoded image's content hash up
first, and skips the total-storage comparison entirely when the server already holds those bytes,
so re-uploading a map to realign it (or picking up art another room already has) never trips the
limit even sitting right at the cap. Getting this wrong would have meant a room unable to
re-align its own map once the server filled up, which is the case
`TestUploadAsset_RejectsPastTotalStorageLimit` exercises end to end.

Both rejections answer with a plain, single-clause error in the house style (`writeError`'s
existing convention) rather than anything explaining why the limit exists — that reasoning is a
comment in the Go, not the toast a Player sees. The total-storage one is a fresh `507 Insufficient
Storage` rather than reusing `400`, since it's nothing about what was uploaded and everything about
the server itself.

No web admin UI, as the story asked: changing either number means editing `longtable.toml` and
restarting, same as every other setting.
