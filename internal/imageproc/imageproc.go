// Package imageproc turns whatever a Room Member uploaded into one
// canonical, safe form: decoded to pixels and re-encoded as WebP, with
// nothing else carried across.
//
// The safety argument is the whole point (see the
// room-member-safe-asset-content story). An uploaded file is untrusted
// input that other people's browsers will render, and image containers
// have room for a great deal that isn't pixels — EXIF blocks, colour
// profiles, trailing data after the image ends, whole second files
// appended to a valid PNG. Decoding to an image.Image and encoding
// again from those pixels leaves all of it behind, because nothing but
// the pixel grid survives the round trip. That also means the bytes we
// serve are bytes this program produced, never bytes someone uploaded.
//
// WebP as the single output format comes from ADR-0005: map images are
// large and want real lossy compression, and having exactly one output
// format keeps the serving path, the mime type and the library preview
// from having to care what was uploaded.
package imageproc

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/gen2brain/webp"
)

// Quality for the lossy WebP encode. 90 is high enough that map art
// doesn't visibly degrade, while still cutting a multi-megabyte PNG map
// to a fraction of its size — which is the reason WebP was chosen.
// Alpha is stored losslessly by the encoder regardless, so cut-out
// token art keeps clean edges.
const Quality = 90

// MaxPixels caps the decoded image, independent of how few bytes it
// arrived in. A "decompression bomb" — a tiny file describing an
// enormous canvas — would otherwise have us allocate its full
// width×height×4 before anything else got a say. 64MP is far beyond any
// real battle map (8000×8000) while keeping the worst case around
// 256MB.
const MaxPixels = 64 << 20

var (
	// ErrUnsupportedFormat covers both "we don't accept this kind of
	// image" and "this isn't an image at all" on purpose: from the
	// uploader's side they're the same mistake, and distinguishing them
	// would mean reporting on how far a decode got through a hostile
	// file.
	ErrUnsupportedFormat = errors.New("unsupported image format")
	ErrTooLarge          = errors.New("image dimensions too large")
)

// Result is a re-encoded image, ready to store.
type Result struct {
	// Data is the WebP encoding — the only bytes that should ever be
	// written to the blob store or served.
	Data []byte

	Width  int
	Height int

	// SourceFormat is what arrived ("png", "jpeg", "gif", "webp"), kept
	// for the log line and the upload response rather than for anything
	// behavioural.
	SourceFormat string

	// Frames is how many frames the source held, and Animated whether
	// that was more than one. Only the first frame survives: the canvas
	// draws a still image, so an animated upload is accepted and
	// flattened rather than rejected — but the caller is expected to say
	// so, since silently dropping the other 11 frames of someone's
	// goblin would otherwise look like a bug.
	Frames   int
	Animated bool
}

// Reencode decodes r and returns its first frame as WebP. Everything
// that isn't pixels is dropped in the process.
func Reencode(r io.Reader) (Result, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Result{}, err
	}

	// Format comes from sniffing the content, never from the filename or
	// the client's Content-Type: both are attacker-controlled, and a
	// .png that is really something else is exactly the case this
	// package exists to catch.
	format, ok := detectFormat(data)
	if !ok {
		return Result{}, ErrUnsupportedFormat
	}

	if err := checkDimensions(data, format); err != nil {
		return Result{}, err
	}

	first, frames, err := decodeFirstFrame(data, format)
	if err != nil {
		// A file that sniffed as an image but won't decode is either
		// corrupt or crafted; either way it isn't something to store.
		return Result{}, fmt.Errorf("%w: %v", ErrUnsupportedFormat, err)
	}

	var out bytes.Buffer
	if err := webp.Encode(&out, first, webp.Options{Quality: Quality}); err != nil {
		return Result{}, fmt.Errorf("encode webp: %w", err)
	}

	bounds := first.Bounds()
	return Result{
		Data:         out.Bytes(),
		Width:        bounds.Dx(),
		Height:       bounds.Dy(),
		SourceFormat: format,
		Frames:       frames,
		Animated:     frames > 1,
	}, nil
}

// detectFormat reports the image format of data, limited to the four
// input formats accepted. http.DetectContentType implements the WHATWG
// sniffing algorithm, so this agrees with what a browser would conclude
// about the same bytes.
func detectFormat(data []byte) (string, bool) {
	switch http.DetectContentType(data) {
	case "image/png":
		return "png", true
	case "image/jpeg":
		return "jpeg", true
	case "image/gif":
		return "gif", true
	case "image/webp":
		return "webp", true
	default:
		return "", false
	}
}

// checkDimensions reads just the header, so an oversized image is
// refused before its pixels are ever allocated.
func checkDimensions(data []byte, format string) error {
	var (
		cfg image.Config
		err error
	)
	if format == "webp" {
		cfg, err = webp.DecodeConfig(bytes.NewReader(data))
	} else {
		cfg, _, err = image.DecodeConfig(bytes.NewReader(data))
	}
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnsupportedFormat, err)
	}

	if cfg.Width <= 0 || cfg.Height <= 0 {
		return fmt.Errorf("%w: image has no area", ErrUnsupportedFormat)
	}
	if int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return fmt.Errorf("%w: %dx%d exceeds %d pixels", ErrTooLarge, cfg.Width, cfg.Height, MaxPixels)
	}
	return nil
}

// decodeFirstFrame returns the image to keep, plus how many frames the
// source actually held. GIF and WebP are decoded through their
// multi-frame entry points purely to learn that count — a still image
// comes back as a one-frame animation, which needs no special case.
func decodeFirstFrame(data []byte, format string) (image.Image, int, error) {
	switch format {
	case "png":
		img, err := png.Decode(bytes.NewReader(data))
		return img, 1, err
	case "jpeg":
		// Left exactly as decoded even though this is also a YCbCr image:
		// JPEG's YCbCr is full-range by JFIF, which is precisely what
		// Go's conversion implements. Running it through the WebP fix
		// below would break it. See expandVideoRange.
		img, err := jpeg.Decode(bytes.NewReader(data))
		return img, 1, err
	case "gif":
		all, err := gif.DecodeAll(bytes.NewReader(data))
		if err != nil {
			return nil, 0, err
		}
		if len(all.Image) == 0 {
			return nil, 0, errors.New("gif has no frames")
		}
		return all.Image[0], len(all.Image), nil
	case "webp":
		all, err := webp.DecodeAll(bytes.NewReader(data))
		if err != nil {
			return nil, 0, err
		}
		if len(all.Image) == 0 {
			return nil, 0, errors.New("webp has no frames")
		}
		return expandVideoRange(all.Image[0]), len(all.Image), nil
	default:
		return nil, 0, ErrUnsupportedFormat
	}
}

// expandVideoRange converts a WebP-decoded YCbCr image to NRGBA using
// the BT.601 *limited-range* matrix, and returns anything else
// untouched.
//
// This exists because of a trap worth knowing about before touching it.
// Lossy WebP is VP8, whose luma is studio swing: black sits at 16 and
// white at 235, not 0 and 255. Go's image.YCbCr — and therefore
// anything that reads one through At() or draw.Draw — implements the
// *full-range* JFIF conversion instead, because that's what JPEG uses.
// Decode a lossy WebP and read its pixels the obvious way and every
// value comes back squeezed toward mid-grey by about 14% (240 reads as
// 222, 32 as 43), which then gets baked in permanently when we re-encode
// from those pixels.
//
// Verified against Chrome, which decodes the same files with real
// libwebp: with this conversion the round trip lands within a couple of
// levels of the original, and without it every WebP re-upload loses
// contrast. The imageproc tests carry the browser-checked numbers.
//
// Only WebP gets this treatment. JPEG produces the same Go type from
// genuinely full-range data, so applying it there would introduce the
// very error this removes.
func expandVideoRange(img image.Image) image.Image {
	type ycbcrPlanes interface {
		YOffset(x, y int) int
		COffset(x, y int) int
	}

	planes, ok := img.(ycbcrPlanes)
	if !ok {
		// Lossless WebP decodes straight to an RGBA-family image, which
		// never went through YCbCr and needs nothing done to it.
		return img
	}

	var (
		yy, cb, cr []uint8
		alpha      *image.NYCbCrA
	)
	switch src := img.(type) {
	case *image.NYCbCrA:
		yy, cb, cr, alpha = src.Y, src.Cb, src.Cr, src
	case *image.YCbCr:
		yy, cb, cr = src.Y, src.Cb, src.Cr
	default:
		return img
	}

	bounds := img.Bounds()
	out := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			yi := planes.YOffset(x, y)
			ci := planes.COffset(x, y)

			// The standard BT.601 limited-range matrix.
			luma := 1.164 * (float64(yy[yi]) - 16)
			u := float64(cb[ci]) - 128
			v := float64(cr[ci]) - 128

			a := uint8(0xff)
			if alpha != nil {
				a = alpha.A[alpha.AOffset(x, y)]
			}
			out.SetNRGBA(x, y, color.NRGBA{
				R: clamp8(luma + 1.596*v),
				G: clamp8(luma - 0.391*u - 0.813*v),
				B: clamp8(luma + 2.017*u),
				A: a,
			})
		}
	}
	return out
}

func clamp8(v float64) uint8 {
	switch {
	case v <= 0:
		return 0
	case v >= 255:
		return 255
	default:
		return uint8(v + 0.5)
	}
}

// WebPFilename rewrites an uploaded filename's extension to .webp, so
// what a Room Member sees in the library still matches what they
// uploaded while naming the format actually stored. The name is only
// ever display text — the blob is addressed by content hash — but a
// "goblin.png" that is really WebP is a small lie worth not telling.
func WebPFilename(filename string) string {
	base := path.Base(strings.ReplaceAll(filename, `\`, "/"))
	base = strings.TrimSuffix(base, path.Ext(base))
	if base == "" || base == "." {
		base = "image"
	}
	return base + ".webp"
}

// MimeType is the content type every stored asset now has.
const MimeType = "image/webp"
