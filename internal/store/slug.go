package store

import (
	"crypto/rand"
	"errors"
	"strings"
)

// slugAlphabet drops 0/o/1/l/i so a slug read aloud or copied off a screen
// can't come back ambiguous.
const slugAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

const slugLength = 6

// newSlug returns a random 6-character room slug that doesn't happen to spell
// something offensive. A slug is the one piece of a room everybody says out
// loud and pastes into a group chat, so handing a table a slur — even at a few
// in a hundred thousand — is worth a retry loop to avoid.
func newSlug() (string, error) {
	// A rejection is rare enough that this bound only trips if rand.Read starts
	// returning a constant; the alternative, an unbounded loop, would hang the
	// request instead of failing it.
	const maxAttempts = 100
	for attempt := 0; attempt < maxAttempts; attempt++ {
		buf := make([]byte, slugLength)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for i, b := range buf {
			buf[i] = slugAlphabet[int(b)%len(slugAlphabet)]
		}
		slug := string(buf)
		if !isOffensiveSlug(slug) {
			return slug, nil
		}
	}
	return "", errors.New("could not generate an inoffensive room slug")
}

// leetToLetters maps each digit in slugAlphabet onto the letter it reads as, so
// "a55" is caught by the same blocklist entry that catches "ass". It over-maps
// on purpose: 9 rarely stands in for g, but a false rejection costs one extra
// round of the loop and a miss costs a room named after a slur.
var leetToLetters = strings.NewReplacer(
	"2", "z",
	"3", "e",
	"4", "a",
	"5", "s",
	"6", "g",
	"7", "t",
	"8", "b",
	"9", "g",
)

// slugBlocklist is the set of words a slug must not contain, before or after
// de-leeting. Two rules keep it short and keep every entry load-bearing —
// TestSlugBlocklist_EntriesAreReachable enforces both:
//
//   - Nothing spelled with i, l or o is here, because slugAlphabet has none of
//     those and no digit reads as one. That alone rules out most of the words
//     you'd expect on a list like this.
//   - Nothing longer than slugLength is here, since it could never fit.
//
// The list is deliberately limited to slurs and hard profanity. Mild words
// (damn, crap, turd) are left off: each entry raises the rejection rate for
// every room ever created, and a room called "turd4x" is a joke, not an
// incident.
var slugBlocklist = []string{
	"anus",
	"arse",
	"ass",
	"cum",
	"cunt",
	"dyke",
	"fag",
	"fck",
	"fuck",
	"fuk",
	"gash",
	"gyp",
	"jap",
	"kkk",
	"kraut",
	"kunt",
	"negr",
	"phuck",
	"queer",
	"rape",
	"skank",
	"snatch",
	"spaz",
	"tranny",
	"twat",
	"wank",
}

// isOffensiveSlug reports whether slug contains a blocklisted word, reading
// digits as the letters they stand in for.
func isOffensiveSlug(slug string) bool {
	candidates := []string{slug, leetToLetters.Replace(slug)}
	for _, candidate := range candidates {
		for _, word := range slugBlocklist {
			if strings.Contains(candidate, word) {
				return true
			}
		}
	}
	return false
}
