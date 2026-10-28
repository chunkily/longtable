//go:build ignore

// Generates portrait.png, per the recipe in README.md:
//
//	go run gen-portrait.go
//
// A 40x120 picture in three 40x40 bands — red over green over blue — so a
// spec can tell which square of it the token cropper kept: the top one
// by default, since tall token art is framed from the top, and another
// once the frame has been moved. The band colours are this fixture's own;
// no other fixture uses them.
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
)

func main() {
	const w, h = 40, 120
	bands := []color.RGBA{
		{R: 0xd8, G: 0x24, B: 0x24, A: 0xff},
		{R: 0x24, G: 0xb4, B: 0x3c, A: 0xff},
		{R: 0x24, G: 0x3c, B: 0xd8, A: 0xff},
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, bands[y/(h/3)])
		}
	}

	out, err := os.Create("portrait.png")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	if err := png.Encode(out, img); err != nil {
		log.Fatal(err)
	}
}
