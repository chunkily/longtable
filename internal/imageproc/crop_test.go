package imageproc

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"testing"
)

// Framing token art is a crop applied during the re-encode, so what's
// stored is already the square the uploader chose.

// bandedPortrait is a 60×180 picture in three 60×60 bands, red over
// green over blue — a portrait whose top, middle and bottom squares can
// each be told apart after a lossy round trip.
func bandedPortrait() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 60, 180))
	bands := []color.NRGBA{{220, 30, 30, 255}, {30, 180, 60, 255}, {30, 60, 220, 255}}
	for y := range 180 {
		for x := range 60 {
			img.Set(x, y, bands[y/60])
		}
	}
	return img
}

// strongest names the channel that dominates a pixel, which survives
// WebP's lossy encode where exact values don't.
func strongest(c color.Color) string {
	r, g, b, _ := c.RGBA()
	switch {
	case r > g && r > b:
		return "red"
	case g > r && g > b:
		return "green"
	default:
		return "blue"
	}
}

func TestReencodeCrop_KeepsExactlyTheChosenSquare(t *testing.T) {
	src := encodePNG(t, bandedPortrait())

	for _, c := range []struct {
		crop Crop
		want string
	}{
		{Crop{X: 0, Y: 0, Size: 60}, "red"},
		{Crop{X: 0, Y: 60, Size: 60}, "green"},
		{Crop{X: 0, Y: 120, Size: 60}, "blue"},
		// Zoomed in: a 20px square from the middle of the green band.
		{Crop{X: 20, Y: 80, Size: 20}, "green"},
	} {
		got, err := ReencodeCrop(bytes.NewReader(src), c.crop)
		if err != nil {
			t.Fatalf("ReencodeCrop(%+v): %v", c.crop, err)
		}
		if got.Width != c.crop.Size || got.Height != c.crop.Size {
			t.Fatalf("crop %+v came out %dx%d, want a %d square", c.crop, got.Width, got.Height, c.crop.Size)
		}
		img := decodeWebP(t, got.Data)
		// Corners as well as the middle: a crop one band off would still
		// match in the middle of a big band but not at its edges.
		for _, p := range []image.Point{{1, 1}, {c.crop.Size / 2, c.crop.Size / 2}, {c.crop.Size - 2, c.crop.Size - 2}} {
			if s := strongest(img.At(p.X, p.Y)); s != c.want {
				t.Errorf("crop %+v at %v is %s, want %s", c.crop, p, s, c.want)
			}
		}
	}
}

// Transparency inside the frame stays transparent: cut-out token art is
// the common case, and compositing it onto black would ruin every one.
func TestReencodeCrop_KeepsTransparency(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 40, 80)) // zero value: transparent
	got, err := ReencodeCrop(bytes.NewReader(encodePNG(t, img)), Crop{X: 0, Y: 20, Size: 40})
	if err != nil {
		t.Fatalf("ReencodeCrop: %v", err)
	}
	if _, _, _, a := decodeWebP(t, got.Data).At(20, 20).RGBA(); a != 0 {
		t.Fatalf("alpha %d inside the crop, want 0", a)
	}
}

func TestReencodeCrop_RefusesASquareOutsideTheImage(t *testing.T) {
	src := encodePNG(t, bandedPortrait())
	for _, crop := range []Crop{
		{X: 0, Y: 0, Size: 0},
		{X: -1, Y: 0, Size: 60},
		{X: 0, Y: 121, Size: 60}, // one pixel past the bottom
		{X: 1, Y: 0, Size: 60},   // one pixel past the right
		{X: 0, Y: 0, Size: 61},   // wider than the image
	} {
		if _, err := ReencodeCrop(bytes.NewReader(src), crop); !errors.Is(err, ErrBadCrop) {
			t.Errorf("ReencodeCrop(%+v) = %v, want ErrBadCrop", crop, err)
		}
	}
}
