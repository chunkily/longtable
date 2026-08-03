package imageproc

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"testing"

	"github.com/gen2brain/webp"
)

// Grid alignment is padding and cropping applied during the re-encode
// everything already goes through, so the stored map is aligned and no
// offset has to be remembered, passed around or applied at render time.

func decodeWebP(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode webp: %v", err)
	}
	return img
}

func TestReencodeOffset_PadsTopLeft(t *testing.T) {
	src := encodePNG(t, gradient(40, 30))

	got, err := ReencodeOffset(bytes.NewReader(src), Offset{X: 12, Y: 7})
	if err != nil {
		t.Fatalf("ReencodeOffset: %v", err)
	}
	if got.Width != 52 || got.Height != 37 {
		t.Fatalf("result is %dx%d, want 52x37", got.Width, got.Height)
	}

	img := decodeWebP(t, got.Data)
	// The padding has to be transparent rather than black — a map nudged
	// right should show the table underneath, not a bar.
	if _, _, _, a := img.At(2, 2).RGBA(); a != 0 {
		t.Fatalf("padding at (2,2) has alpha %d, want 0", a)
	}
	if _, _, _, a := img.At(20, 20).RGBA(); a == 0 {
		t.Fatal("the image itself came back transparent")
	}
}

// Positive pads, negative crops — the same field either way, because
// from the aligner's side both are "shift where the art starts".
func TestReencodeOffset_CropsOnNegative(t *testing.T) {
	// A white margin over a black field, so "did the margin go?" is a
	// question lossy WebP can still answer. Exact pixel values would not
	// survive: VP8's luma is studio swing and reading it back through Go's
	// full-range conversion squeezes everything toward mid-grey — the trap
	// expandVideoRange exists for.
	src := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for y := range 30 {
		for x := range 40 {
			shade := color.RGBA{A: 255}
			if x < 8 || y < 6 {
				shade = color.RGBA{R: 255, G: 255, B: 255, A: 255}
			}
			src.Set(x, y, shade)
		}
	}

	got, err := ReencodeOffset(bytes.NewReader(encodePNG(t, src)), Offset{X: -8, Y: -6})
	if err != nil {
		t.Fatalf("ReencodeOffset: %v", err)
	}
	if got.Width != 32 || got.Height != 24 {
		t.Fatalf("result is %dx%d, want 32x24", got.Width, got.Height)
	}

	// The white margin is what was cropped, so the origin is now field.
	img := decodeWebP(t, got.Data)
	if r, _, _, _ := img.At(0, 0).RGBA(); r>>8 > 128 {
		t.Fatalf("origin pixel is still light (r=%d) — the margin wasn't the part removed", r>>8)
	}
	if r, _, _, _ := img.At(20, 15).RGBA(); r>>8 > 128 {
		t.Fatalf("the field came back light (r=%d)", r>>8)
	}
}

func TestReencodeOffset_RejectsACropOfEverything(t *testing.T) {
	src := encodePNG(t, gradient(20, 20))

	if _, err := ReencodeOffset(bytes.NewReader(src), Offset{X: -20}); !errors.Is(err, ErrBadOffset) {
		t.Fatalf("err = %v, want ErrBadOffset", err)
	}
	if _, err := ReencodeOffset(bytes.NewReader(src), Offset{Y: -25}); !errors.Is(err, ErrBadOffset) {
		t.Fatalf("err = %v, want ErrBadOffset", err)
	}
}

// The header check at the top of the decode bounds the upload; padding
// happens after it and makes the image bigger, so it needs its own.
func TestReencodeOffset_PaddingCantExceedThePixelCap(t *testing.T) {
	src := encodePNG(t, gradient(8, 8))

	if _, err := ReencodeOffset(bytes.NewReader(src), Offset{X: 8200, Y: 8200}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

func TestReencode_LeavesTheImageAloneByDefault(t *testing.T) {
	src := encodePNG(t, gradient(40, 30))

	got, err := Reencode(bytes.NewReader(src))
	if err != nil {
		t.Fatalf("Reencode: %v", err)
	}
	if got.Width != 40 || got.Height != 30 {
		t.Fatalf("result is %dx%d, want the original 40x30", got.Width, got.Height)
	}
}

// A padded region on an image that had no alpha at all is the case where
// getting the destination type wrong turns transparency into black.
func TestReencodeOffset_PaddingSurvivesAnOpaqueSource(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			img.Set(x, y, color.RGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}

	got, err := ReencodeOffset(bytes.NewReader(encodePNG(t, img)), Offset{X: 6, Y: 6})
	if err != nil {
		t.Fatalf("ReencodeOffset: %v", err)
	}
	if _, _, _, a := decodeWebP(t, got.Data).At(1, 1).RGBA(); a != 0 {
		t.Fatalf("padding alpha = %d, want 0", a)
	}
}
