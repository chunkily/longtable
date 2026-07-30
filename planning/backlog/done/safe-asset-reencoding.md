---
title: Re-encode every uploaded asset
created: 2026-07-30
tags: [assets, safety]
story: room-member-safe-asset-content
---

Decode and re-encode every uploaded image server-side, so nothing that isn't pixels can reach
another Room Member's browser. Accept only PNG, JPEG, WebP and GIF; reject anything that doesn't
decode. Settled in [ADR-0005](../../decisions/0005-webp-reencoding-library.md), which had been
accepted but unbuilt — this item was created when the work started, having lived only as a user
story until then.

## What shipped

`internal/imageproc` decodes an upload and re-encodes it as WebP (lossy q90), and the upload path
stores only that. Format comes from sniffing the content, never the filename or the client's
`Content-Type`. Anything that doesn't decode is a 400. Animated GIFs and WebPs are accepted and
flattened to their first frame, with `flattened: true` on the response so the uploader can be
told rather than left wondering why their goblin stopped moving.

Everything else falls out of the round trip: EXIF, colour profiles, trailing data after `IEND`,
whole files appended to a valid PNG — none of it survives being decoded to a pixel grid and
encoded again. The content hash is now taken over the *re-encoded* bytes rather than the upload,
which also means dedup describes what people actually get served, so the same map uploaded once as
PNG and once as JPEG resolves to one stored file.

Two things worth knowing before touching this:

- **Lossy WebP luma is studio swing** (black at 16, white at 235), and Go's `image.YCbCr`
  implements the full-range JFIF conversion instead, because that's what JPEG uses. Decoding a
  lossy WebP and reading its pixels the obvious way squeezes every value toward mid-grey by about
  14% — 240 reads as 222, 32 as 43 — and re-encoding from those pixels bakes it in. So a WebP
  *re-upload* would lose contrast every time, permanently. `expandVideoRange` applies the BT.601
  limited-range matrix to WebP-decoded images only; JPEG must not go through it, despite producing
  the same Go type from genuinely full-range data. The expected values in the tests were measured
  from Chrome decoding the identical bytes with real libwebp, so they're ground truth rather than
  the conversion grading itself.
- **A decompression bomb is checked on the header**, before any pixels are allocated: dimensions
  come from `DecodeConfig` and anything past 64MP is refused. A tiny file describing a
  30000×30000 canvas is otherwise a ~3.6GB allocation.

The dependency also isn't quite what ADR-0005 describes any more — see the note appended to that
decision.

## Related user stories

- [room-member-safe-asset-content](../../user-stories/room-member-safe-asset-content.md)
