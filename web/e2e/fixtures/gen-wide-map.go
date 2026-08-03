//go:build ignore

// Generates wide-map.png, per the recipe in README.md:
//
//	go run gen-wide-map.go
//
// Wide rather than 8x8 on purpose — it's the one fixture whose *shape*
// is the thing under test, since the assets page reads dimensions to
// question whether a staged file is really what the open tab says.
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
)

func main() {
	const w, h = 40, 12

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 0x4a, G: 0x2f, B: 0x6b, A: 0xff})
		}
	}

	out, err := os.Create("wide-map.png")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	if err := png.Encode(out, img); err != nil {
		log.Fatal(err)
	}
}
