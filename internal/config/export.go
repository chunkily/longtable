package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Export renders settings as commented TOML.
//
// This is the file coming back, but as *output* rather than as the
// source of truth — the server never reads it, so there is no second
// writer to disagree with the database and no comment to preserve
// through a round trip. Both of those are what made keeping the old
// config file alongside a settings UI unworkable ([ADR-0009]).
//
// What it gives back is the one property the file was quietly providing
// and a sequence of `config set` commands is not: a single artefact a
// Host can read, diff, keep in version control and replay on another
// machine.
//
// [ADR-0009]: ../../planning/decisions/0009-admin-ui-and-database-config.md
func Export(values map[string]string) string {
	var b strings.Builder

	b.WriteString("# Longtable settings, written by `longtable config export`.\n")
	b.WriteString("#\n")
	b.WriteString("# Load it back with `longtable config import <file>`. Longtable does not\n")
	b.WriteString("# read this file when it starts — the settings live in the database, and\n")
	b.WriteString("# this is a copy of them.\n")

	// Top-level keys first, then tables. TOML puts every bare key above
	// the first [section] header, so anything written after one belongs
	// to it whether that was meant or not.
	for _, s := range Registry {
		if strings.Contains(s.Key, ".") {
			continue
		}
		writeSetting(&b, s, s.Key, values)
	}

	for _, section := range sections() {
		fmt.Fprintf(&b, "\n[%s]\n", section)
		for _, s := range Registry {
			prefix, name, found := strings.Cut(s.Key, ".")
			if !found || prefix != section {
				continue
			}
			writeSetting(&b, s, name, values)
		}
	}

	return b.String()
}

func writeSetting(b *strings.Builder, s Setting, name string, values map[string]string) {
	b.WriteString("\n")
	for _, line := range wrap(s.Summary, 68) {
		fmt.Fprintf(b, "# %s\n", line)
	}
	value := values[s.Key]
	if value == "" {
		value = s.Default
	}
	if !s.Live {
		b.WriteString("# Changing this needs a restart.\n")
	}
	fmt.Fprintf(b, "%s = %q\n", name, value)
}

// sections is every dotted prefix in the registry, in first-appearance
// order so an export's shape is the registry's rather than the
// alphabet's.
func sections() []string {
	var order []string
	seen := map[string]bool{}
	for _, s := range Registry {
		prefix, _, found := strings.Cut(s.Key, ".")
		if !found || seen[prefix] {
			continue
		}
		seen[prefix] = true
		order = append(order, prefix)
	}
	return order
}

// Import reads what Export wrote, as raw key/value pairs.
//
// Values come back as strings and unvalidated on purpose: every one of
// them still has to go through the registry's Parse, so an imported file
// cannot put a value into the database that `config set` would have
// refused.
func Import(data []byte) (map[string]string, error) {
	var tree map[string]any
	if err := toml.Unmarshal(data, &tree); err != nil {
		var decErr *toml.DecodeError
		if errors.As(err, &decErr) {
			// String() draws the offending line under the summary, which is
			// the difference between finding a mistake and hunting for it.
			return nil, fmt.Errorf("%s", decErr.String())
		}
		return nil, err
	}

	values := map[string]string{}
	flatten("", tree, values)
	return values, nil
}

// flatten turns TOML's nesting back into the dotted keys the registry
// uses.
func flatten(prefix string, tree map[string]any, out map[string]string) {
	for key, value := range tree {
		full := key
		if prefix != "" {
			full = prefix + "." + key
		}
		if nested, ok := value.(map[string]any); ok {
			flatten(full, nested, out)
			continue
		}
		// Everything is stringified rather than type-switched: a Host who
		// wrote `departure_grace = 30` without quotes meant something, and
		// Parse is where it gets judged.
		out[full] = fmt.Sprint(value)
	}
}

// wrap breaks a summary into comment-width lines.
func wrap(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	lines := []string{words[0]}
	for _, word := range words[1:] {
		last := len(lines) - 1
		if len(lines[last])+1+len(word) <= width {
			lines[last] += " " + word
			continue
		}
		lines = append(lines, word)
	}
	return lines
}
