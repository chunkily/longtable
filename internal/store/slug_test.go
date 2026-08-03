package store

import (
	"strings"
	"testing"
)

// A slug is only unambiguous if every character came from the alphabet, and
// only safe to hand out if the generator's filter actually ran.
func TestNewSlug_UsesAlphabetAndAvoidsBlockedWords(t *testing.T) {
	for i := 0; i < 2000; i++ {
		slug, err := newSlug()
		if err != nil {
			t.Fatalf("newSlug: %v", err)
		}
		if len(slug) != slugLength {
			t.Fatalf("slug %q has length %d, want %d", slug, len(slug), slugLength)
		}
		for _, r := range slug {
			if !strings.ContainsRune(slugAlphabet, r) {
				t.Fatalf("slug %q contains %q, which is not in the alphabet", slug, r)
			}
		}
		if isOffensiveSlug(slug) {
			t.Fatalf("newSlug returned a blocklisted slug: %q", slug)
		}
	}
}

func TestIsOffensiveSlug(t *testing.T) {
	blocked := []string{
		"fuck7a", // straight through
		"2fagxy", // the word sits in the middle
		"xxxass", // and at the end
		"f4gzzz", // 4 reads as a
		"a55hat", // 5 reads as s
		"cun7zz", // 7 reads as t
		"r4p3xx", // two substitutions at once
	}
	for _, slug := range blocked {
		if !isOffensiveSlug(slug) {
			t.Errorf("isOffensiveSlug(%q) = false, want true", slug)
		}
	}

	allowed := []string{"btgrza", "h3rmz2", "zzzzzz", "kb4y7n"}
	for _, slug := range allowed {
		if isOffensiveSlug(slug) {
			t.Errorf("isOffensiveSlug(%q) = true, want false", slug)
		}
	}

	// Matching is on substrings, not words, so a slug that merely contains one
	// — "class4" — goes too. That is the intended trade: a slug is a meaningless
	// string, so there is no innocent one worth defending, and a false rejection
	// costs one more turn of the generator loop while a miss costs a table.
	if !isOffensiveSlug("class4") {
		t.Error("isOffensiveSlug(\"class4\") = false, want true: matching is on substrings")
	}
}

// Every blocklist entry has to be reachable by the generator, or it is dead
// weight that reads like protection. slugAlphabet has no i/l/o and no digit
// standing in for one, and a slug is only slugLength characters long.
func TestSlugBlocklist_EntriesAreReachable(t *testing.T) {
	for _, word := range slugBlocklist {
		if len(word) > slugLength {
			t.Errorf("blocklist entry %q is longer than a slug, so it can never match", word)
		}
		for _, r := range word {
			if !strings.ContainsRune(slugAlphabet, r) {
				t.Errorf("blocklist entry %q contains %q, which no slug can spell", word, r)
			}
		}
	}
}
