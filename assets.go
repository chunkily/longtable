// Package assets embeds the built SvelteKit frontend (web/build) into the
// Go binary so the server has no external files to ship alongside it.
//
// Run `npm run build` inside web/ before `go build` — the embed directive
// below fails if web/build doesn't exist yet.
package assets

import "embed"

//go:embed all:web/build
var Dist embed.FS

// DistDir is the subdirectory within Dist that the embedded files live
// under, since go:embed preserves the full relative path.
const DistDir = "web/build"
