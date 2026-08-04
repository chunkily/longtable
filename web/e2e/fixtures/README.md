# E2E image fixtures

Real, small PNGs for the specs that upload something. Load them with the `fixture()` helper in
`../fixtures.ts`, which resolves paths against the module rather than the working directory.

All 8x8 except `wide-map.png`, which is 40x12 because its _shape_ is what it's for: the assets
page reads a staged file's dimensions and questions whether it's really the kind the open tab
says, and only a decidedly non-square image exercises that. `gen-wide-map.go` is the program that
produced it, kept beside it so the next odd-shaped fixture doesn't start from scratch.

Two rules, both of which have already cost someone an afternoon:

**They have to be genuinely encoded images.** `imageproc.Reencode` sniffs the content and answers
400 for anything that isn't one, so a file produced by hand-editing another fixture's bytes fails
in a way that reads like an application bug: the upload 400s, the server logs nothing (the
request never gets past the decode), and the only symptom on screen is an asset picker that stays
empty — the error toast expires before a 5s locator timeout does. Encode new ones; don't edit
these.

**Every fixture needs pixels no other fixture has.** Assets are content-addressed and
`web/.e2e-data/longtable.db` is never reset between runs, so two fixtures with identical content
resolve to _one_ asset row — under whichever filename got there first, months ago, in an
unrelated spec. A flat colour nobody else used is enough. Current fixtures are one flat colour
each and their hashes are distinct.

Uploading by path (`setInputFiles(fixture('goblin.png'))`) sends the file's real basename, which
keeps the name a spec asserts on (`goblin.webp`, after the WebP re-encode) tied to the bytes that
produce it. That lockstep is the point of keeping these as files. The one spec that deliberately
breaks it — re-uploading identical content under a different name to prove dedup — passes an
explicit `{ name, buffer }` instead, and says so.

To add one:

```go
// go run this, then save stdout to a new file here
img := image.NewRGBA(image.Rect(0, 0, 8, 8))
for y := range 8 {
	for x := range 8 {
		img.Set(x, y, color.RGBA{R: 0x2f, G: 0x6b, B: 0x3a, A: 0xff}) // pick an unused colour
	}
}
png.Encode(out, img)
```
