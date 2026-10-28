---
title: Frame token art when it's uploaded
created: 2026-10-06
status: done
tags: [assets, tokens, upload, playtest]
story: room-member-frames-token-art
---

Playtest feedback: people liked bringing their own art, but a portrait can't be squeezed into a
circle without someone deciding what's kept. Let the uploader decide, once, on the assets page.

Staging a non-square file under the **Tokens** tab shows a cropper — the same rule as the map
aligner, and for the same reason: there is no path to `Add to library` that goes around it. It
opens already framed, so a default that's fine is one click:

- the largest square that fits, centred — or anchored to the top for tall art, which is usually
  where the face is
- drag to move it, zoom in to frame a headshot out of a full-body piece
- a circle overlay showing exactly what the token will show, corners included

Square art skips the step; there is nothing to choose.

**The crop is baked into the pixels, the way a grid offset is.** The client sends a rectangle in
*natural* image pixels and `imageproc` crops during the re-encode every upload already goes
through, beside `ReencodeOffset`. The stored WebP comes out square, so the map, the initiative
panel, the library tile and every picker show the same framing and none of them knows a crop
existed. The `MaxPixels` check runs on the result as it does after an offset.

Decided against, with the reasons:

- **Storing the crop as data on `room_asset`.** Re-framable later, but every place token art is
  drawn — canvas, initiative, details panel, hover card, picker — would have to apply it, which is
  the natural-versus-display-pixel conversion `grid-zoom.ts` warns about written five times over.
  It would also leave the uncropped original on disk and served, including whatever was outside
  the circle that nobody at the table saw — a Host moderating it would be judging something other
  than what was shown.
- **Per-token crops.** The same art framed differently on two tokens is real but rare, and it puts
  the crop on the protocol and the token dialog for a case nobody has asked for.
- **An optional cropper with an automatic centre crop.** The aligner's argument applies: art
  nobody looked at is a problem that only shows up later, on the map.

Not in this: re-framing art that's already in a library. That means uploading the original again,
and it is a decision rather than an oversight.

- [ ] The cropper's display arithmetic lives in a tested `.ts`, the way `grid-zoom.ts` and
      `pinch.ts` do — natural pixels in the component, everything on screen through one scale
- [ ] `imageproc` crops to a requested rectangle, rejecting one outside the image or of zero size
- [ ] The upload endpoint takes the rectangle for a token upload, validates it, and ignores or
      refuses it for a map
- [ ] Records the upload's source hash — which means
      [record-upload-source-hash](record-upload-source-hash.md) lands first or in the same change
- [ ] Every string on it goes through `longtable-copy`

Builds on [token-art-keeps-its-proportions](token-art-keeps-its-proportions.md), which stays as
the fallback for art that never passed through here.

## What shipped

Token art that isn't square gets a cropper on the assets page, required the way the map aligner
is: `Add to library` stays disabled until the picture has loaded and been framed. It opens on the
largest square — centred across wide art, **anchored to the top** of tall art — with the token's
circle drawn over the picture and everything outside it dimmed. The circle drags (pointer, or
arrow keys at a twentieth of the frame per press, Shift for one pixel) and a `Zoom` slider shrinks
it about its centre, down to a 32px frame. Square art shows no cropper at all.

The square goes up as `cropX`/`cropY`/`cropSize` in natural pixels, and `imageproc.ReencodeCrop`
cuts it out during the re-encode — the same transform slot the grid offset uses, which the
decode/encode path was refactored into (`reencode(r, transform)`). The stored art is square, so
the map, the initiative panel, the library and the pickers agree without knowing a crop happened.

Things worth not rediscovering:

- **The server refuses a crop that isn't for token art** — on a map, with no kind, alongside a grid
  offset, half-specified, or outside the picture — each with its own message. The upload's kind is
  now parsed before the re-encode for that reason.
- **The source hash is recorded either way** ([record-upload-source-hash](record-upload-source-hash.md)
  shipped first), so two framings of one portrait are two assets with one source.
- **The arithmetic is `web/src/lib/crop-frame.ts`**, tested on its own: initial frame, clamping,
  moving, resizing about the centre, zoom ↔ size. The component converts to the screen through
  one `scale` and nothing else.
- **The draggable circle is a `<button>`**, which is what gives it focus, a name ("Move the crop")
  and key events without role-patching. The drag is measured from where it started, so per-move
  rounding can't walk the frame away from the pointer.
- **`portrait.png`** is a new fixture for this — 40x120 in three colour bands — and
  `token-cropper.spec.ts` reads the stored image's size and middle colour in the page to prove
  which square was kept: the top one by default, the middle one after twenty presses of
  ArrowDown.

Not built, as decided: re-framing art already in a library (upload the original again), and
per-token crops.
