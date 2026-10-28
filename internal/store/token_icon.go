package store

import "slices"

// TokenIcons are the icons a token or party member may wear instead of
// art, and the authority on that set — mirrored by TOKEN_ICONS in
// web/src/lib/token-icons.ts, which holds each one's drawing, label and
// search words. TestTokenIcons_MatchTheClientList fails if the two drift.
//
// Keys rather than SVG for the reason IdentityColors gives: what's stored
// is drawn on everyone else's screen, and a column taking any string is
// an injection waiting to happen. A key also means redrawing an icon is a
// client-side edit.
//
// **Namespaced by source** (`lucide:sword`). Lucide came first because it
// was already a dependency; game-icons.net is the planned second pool
// (planning/backlog/game-icons-second-icon-pool.md), and a prefix is what
// lets it arrive as more keys rather than a migration.
//
// A key retired from this list doesn't break the tokens wearing it: they
// draw as having no icon — their initials — until someone picks again.
var TokenIcons = []string{
	"lucide:sword", "lucide:swords", "lucide:shield", "lucide:bow-arrow",
	"lucide:axe", "lucide:hammer", "lucide:pickaxe", "lucide:wand-sparkles",
	"lucide:flask-conical", "lucide:scroll-text", "lucide:book-open", "lucide:crown",
	"lucide:gem", "lucide:coins", "lucide:key", "lucide:lock",
	"lucide:door-open", "lucide:castle", "lucide:tent", "lucide:church",
	"lucide:skull", "lucide:bone", "lucide:ghost", "lucide:eye",
	"lucide:drama", "lucide:venetian-mask", "lucide:heart", "lucide:flame",
	"lucide:zap", "lucide:snowflake", "lucide:droplet", "lucide:cloud-lightning",
	"lucide:tornado", "lucide:sparkles", "lucide:star", "lucide:sun",
	"lucide:moon", "lucide:cat", "lucide:dog", "lucide:bird",
	"lucide:rat", "lucide:fish", "lucide:bug", "lucide:worm",
	"lucide:snail", "lucide:turtle", "lucide:rabbit", "lucide:squirrel",
	"lucide:paw-print", "lucide:feather", "lucide:tree-pine", "lucide:mountain",
	"lucide:user", "lucide:users", "lucide:baby", "lucide:music",
	"lucide:bomb", "lucide:target", "lucide:flag", "lucide:beer",
}

// ValidTokenIcon reports whether k is one of TokenIcons. Empty is valid:
// it's a token with no icon, which is every token from before icons.
func ValidTokenIcon(k string) bool {
	return k == "" || slices.Contains(TokenIcons, k)
}

// ValidTokenColor reports whether k may colour a token: one of the seat
// palette's keys, or empty for the colour the token's id picks. The same
// sixteen as a seat, so the colours a table already knows look the same
// on the map.
func ValidTokenColor(k string) bool {
	return k == "" || slices.Contains(IdentityColors, k)
}
