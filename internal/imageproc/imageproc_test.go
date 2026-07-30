package imageproc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/gen2brain/webp"
)

// gradient is deliberately not a flat colour: a solid block survives
// almost any encoder, so it would pass these tests even if the pixels
// were being mangled.
func gradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// animatedGIF builds a real multi-frame GIF, with the first frame a
// colour the later frames never use — so a test can tell which frame
// came out the other end.
func animatedGIF(t *testing.T, frames int) []byte {
	t.Helper()

	palette := color.Palette{color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}}
	anim := &gif.GIF{}
	for i := range frames {
		frame := image.NewPaletted(image.Rect(0, 0, 8, 8), palette)
		for p := range frame.Pix {
			if i == 0 {
				frame.Pix[p] = 0 // red
			} else {
				frame.Pix[p] = 1 // blue
			}
		}
		anim.Image = append(anim.Image, frame)
		anim.Delay = append(anim.Delay, 10)
	}

	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, anim); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	return buf.Bytes()
}

// decodeResult reads a result back the way a browser would, which is
// not the way Go does it by default: a lossy WebP's luma is studio
// swing, so decoding one and reading At() straight off squeezes every
// value toward mid-grey (see expandVideoRange). Going back through the
// same correction the input path uses is what makes the absolute colour
// assertions below meaningful — and those numbers were checked against
// Chrome decoding the identical bytes with real libwebp, so this isn't
// grading the conversion against itself.
func decodeResult(t *testing.T, r Result) image.Image {
	t.Helper()
	img, err := webp.Decode(bytes.NewReader(r.Data))
	if err != nil {
		t.Fatalf("result is not decodable webp: %v", err)
	}
	return expandVideoRange(img)
}

func TestReencode_AcceptsEverySupportedFormat(t *testing.T) {
	source := gradient(40, 24)

	var webpBuf bytes.Buffer
	if err := webp.Encode(&webpBuf, source); err != nil {
		t.Fatalf("encode webp fixture: %v", err)
	}

	cases := map[string][]byte{
		"png":  encodePNG(t, source),
		"jpeg": encodeJPEG(t, source),
		"gif":  animatedGIF(t, 1),
		"webp": webpBuf.Bytes(),
	}

	for format, data := range cases {
		t.Run(format, func(t *testing.T) {
			result, err := Reencode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("Reencode: %v", err)
			}
			if result.SourceFormat != format {
				t.Fatalf("SourceFormat = %q, want %q", result.SourceFormat, format)
			}
			// Whatever went in, WebP comes out.
			if _, err := webp.DecodeConfig(bytes.NewReader(result.Data)); err != nil {
				t.Fatalf("output is not webp: %v", err)
			}
		})
	}
}

// The point of the exercise: bytes that aren't pixels don't survive.
func TestReencode_DropsEverythingButPixels(t *testing.T) {
	// A valid PNG with a payload appended after IEND. Every image
	// decoder ignores the tail, so it rides along intact through any
	// pipeline that stores what it was given.
	payload := []byte("<script>alert('xss')</script>")
	data := append(encodePNG(t, gradient(16, 16)), payload...)

	result, err := Reencode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Reencode: %v", err)
	}
	if bytes.Contains(result.Data, payload) {
		t.Fatal("appended payload survived re-encoding")
	}
	if bytes.Equal(result.Data, data) {
		t.Fatal("output is the uploaded bytes verbatim")
	}
}

// quadrants paints four saturated blocks, so a test can tell not just
// that colour survived but that it stayed where it was — a flip, a
// rotation or swapped colour channels all move a corner somewhere else.
func quadrants(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	half := size / 2
	for y := range size {
		for x := range size {
			var c color.RGBA
			switch {
			case x < half && y < half:
				c = color.RGBA{R: 220, A: 255}
			case x >= half && y < half:
				c = color.RGBA{G: 200, A: 255}
			case x < half && y >= half:
				c = color.RGBA{B: 220, A: 255}
			default:
				c = color.RGBA{R: 240, G: 240, B: 240, A: 255}
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func TestReencode_KeepsThePicture(t *testing.T) {
	const size = 64
	result, err := Reencode(bytes.NewReader(encodePNG(t, quadrants(size))))
	if err != nil {
		t.Fatalf("Reencode: %v", err)
	}
	if result.Width != size || result.Height != size {
		t.Fatalf("dimensions = %dx%d, want %dx%d", result.Width, result.Height, size, size)
	}

	img := decodeResult(t, result)
	if got := img.Bounds().Dx(); got != size {
		t.Fatalf("decoded width = %d, want %d", got, size)
	}

	// Sampled at the centre of each block, away from the edges where a
	// lossy encode does most of its drifting. Chrome reads these same
	// four colours back within ±1 of the original, so a tolerance of 4
	// leaves room for the encoder without leaving room for the
	// contrast squeeze this used to hide (which showed up as ~18).
	cases := []struct {
		x, y    int
		r, g, b int
		name    string
	}{
		{16, 16, 220, 0, 0, "top-left red"},
		{48, 16, 0, 200, 0, "top-right green"},
		{16, 48, 0, 0, 220, "bottom-left blue"},
		{48, 48, 240, 240, 240, "bottom-right near-white"},
	}
	for _, c := range cases {
		r, g, b, _ := img.At(c.x, c.y).RGBA()
		if abs(int(r>>8)-c.r) > 4 || abs(int(g>>8)-c.g) > 4 || abs(int(b>>8)-c.b) > 4 {
			t.Fatalf("%s at (%d,%d) = (%d,%d,%d), want (%d,%d,%d) ±4",
				c.name, c.x, c.y, r>>8, g>>8, b>>8, c.r, c.g, c.b)
		}
	}
}

// A WebP upload is decoded and re-encoded like anything else, and that
// is where the studio-swing trap bites: read the pixels the obvious way
// and every re-upload of an existing asset loses about 14% of its
// contrast, permanently, because the washed-out values get re-encoded as
// if they were the original.
//
// The expected values here are what Chrome reports for these same
// swatches — measured, not derived — so this fails if expandVideoRange
// is dropped or gets the matrix wrong.
func TestReencode_WebPInputKeepsItsContrast(t *testing.T) {
	swatches := []color.RGBA{
		{R: 240, G: 240, B: 240, A: 255},
		{R: 220, A: 255},
		{G: 200, A: 255},
		{B: 220, A: 255},
		{R: 128, G: 128, B: 128, A: 255},
		{R: 32, G: 32, B: 32, A: 255},
		{R: 200, G: 160, B: 90, A: 255},
	}

	// One 64px band per swatch so each can be sampled far from an edge.
	const band = 64
	source := image.NewRGBA(image.Rect(0, 0, band, band*len(swatches)))
	for i, c := range swatches {
		for y := i * band; y < (i+1)*band; y++ {
			for x := range band {
				source.Set(x, y, c)
			}
		}
	}

	// Round one: PNG in, WebP out — the asset as first uploaded.
	first, err := Reencode(bytes.NewReader(encodePNG(t, source)))
	if err != nil {
		t.Fatalf("Reencode png: %v", err)
	}
	// Round two: that WebP uploaded again, as happens whenever someone
	// downloads an asset and puts it back, or re-uploads to another room.
	second, err := Reencode(bytes.NewReader(first.Data))
	if err != nil {
		t.Fatalf("Reencode webp: %v", err)
	}
	if second.SourceFormat != "webp" {
		t.Fatalf("SourceFormat = %q, want webp", second.SourceFormat)
	}

	for _, result := range []struct {
		name string
		r    Result
	}{{"first pass", first}, {"second pass", second}} {
		img := decodeResult(t, result.r)
		for i, c := range swatches {
			r, g, b, _ := img.At(band/2, i*band+band/2).RGBA()
			if abs(int(r>>8)-int(c.R)) > 6 || abs(int(g>>8)-int(c.G)) > 6 || abs(int(b>>8)-int(c.B)) > 6 {
				t.Fatalf("%s, swatch (%d,%d,%d) came back (%d,%d,%d), off by more than 6",
					result.name, c.R, c.G, c.B, r>>8, g>>8, b>>8)
			}
		}
	}
}

// Token art is cut out against transparency, so alpha surviving a lossy
// encode is not a detail — a token that comes back on a black square is
// unusable.
func TestReencode_PreservesTransparency(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			if x < 16 {
				source.Set(x, y, color.RGBA{R: 200, G: 30, B: 30, A: 255})
			} else {
				source.Set(x, y, color.RGBA{}) // fully transparent
			}
		}
	}

	result, err := Reencode(bytes.NewReader(encodePNG(t, source)))
	if err != nil {
		t.Fatalf("Reencode: %v", err)
	}

	img := decodeResult(t, result)
	if _, _, _, a := img.At(24, 16).RGBA(); a > 0x0fff {
		t.Fatalf("transparent pixel came back with alpha %d", a>>8)
	}
	if _, _, _, a := img.At(4, 16).RGBA(); a < 0xf000 {
		t.Fatalf("opaque pixel came back with alpha %d", a>>8)
	}
}

func TestReencode_FlattensAnimationToTheFirstFrame(t *testing.T) {
	result, err := Reencode(bytes.NewReader(animatedGIF(t, 12)))
	if err != nil {
		t.Fatalf("Reencode: %v", err)
	}
	if !result.Animated || result.Frames != 12 {
		t.Fatalf("Animated = %v, Frames = %d; want true, 12", result.Animated, result.Frames)
	}

	// Frame 1 is red and every later frame is blue, so this is checking
	// which frame was kept, not merely that something was.
	img := decodeResult(t, result)
	r, _, b, _ := img.At(4, 4).RGBA()
	if r < b {
		t.Fatalf("kept a later frame: pixel is (r=%d, b=%d), want the red first frame", r>>8, b>>8)
	}
}

func TestReencode_RejectsWhatIsNotAnImage(t *testing.T) {
	cases := map[string][]byte{
		"plain text":      []byte("this is not an image, it is a sentence"),
		"empty":           {},
		"png header only": []byte("\x89PNG\r\n\x1a\n"),
		"pdf":             []byte("%PDF-1.7\n1 0 obj\n<</Type/Catalog>>\n"),
	}

	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Reencode(bytes.NewReader(data)); !errors.Is(err, ErrUnsupportedFormat) {
				t.Fatalf("err = %v, want ErrUnsupportedFormat", err)
			}
		})
	}
}

// A small file describing an enormous canvas has to be refused on its
// header, before anything allocates width×height×4.
func TestReencode_RejectsDecompressionBomb(t *testing.T) {
	// A PNG header claiming 30000x30000 (900MP), with only a 1x1 image's
	// worth of data behind it — which is what makes it a bomb rather
	// than a big file, and why the size has to be caught on the header
	// instead of on the byte count.
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	data := buf.Bytes()

	// Layout: 8-byte signature, 4-byte length, "IHDR", 13 bytes of data
	// (width, height, then bit depth and friends), 4-byte CRC over the
	// type and data. Patching the dimensions invalidates that CRC, so it
	// has to be recomputed or the decoder rejects the file as corrupt
	// before it ever reads a size.
	copy(data[16:24], []byte{0x00, 0x00, 0x75, 0x30, 0x00, 0x00, 0x75, 0x30})
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))

	if _, err := Reencode(bytes.NewReader(data)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

func TestWebPFilename(t *testing.T) {
	cases := map[string]string{
		"goblin.png":            "goblin.webp",
		"map.jpeg":              "map.webp",
		"already.webp":          "already.webp",
		"no-extension":          "no-extension.webp",
		"":                      "image.webp",
		`C:\Users\a\tavern.png`: "tavern.webp",
		"../../etc/passwd":      "passwd.webp",
	}

	for input, want := range cases {
		if got := WebPFilename(input); got != want {
			t.Fatalf("WebPFilename(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestWebPFilename_StripsDirectoryComponents(t *testing.T) {
	// Browsers don't normally send a path, but the filename is
	// attacker-controlled and ends up in the library UI, so it should
	// never come back out with directories attached.
	for _, input := range []string{"a/b/c.png", `..\..\windows\system32\x.png`} {
		if got := WebPFilename(input); strings.ContainsAny(got, `/\`) {
			t.Fatalf("WebPFilename(%q) = %q, still contains a path separator", input, got)
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
