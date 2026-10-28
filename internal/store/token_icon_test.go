package store

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

// The client draws the icons and the server validates the keys, so the
// two lists have to agree — the same arrangement, and the same check, as
// the seat palette's.
func TestTokenIcons_MatchTheClientList(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "lib", "token-icons.ts"))
	if err != nil {
		t.Fatalf("read the client list: %v", err)
	}
	found := regexp.MustCompile(`key:\s*'([a-z0-9:-]+)'`).FindAllStringSubmatch(string(source), -1)
	client := make([]string, 0, len(found))
	for _, m := range found {
		client = append(client, m[1])
	}
	if !slices.Equal(client, TokenIcons) {
		t.Errorf("client icons %v\nserver icons %v\n— they have to hold the same keys in the same order", client, TokenIcons)
	}
}

func TestValidTokenIconAndColor(t *testing.T) {
	for _, c := range []struct {
		icon, color string
		want        bool
	}{
		{"", "", true},
		{"lucide:skull", "teal", true},
		{"skull", "", false},
		{"lucide:dragon", "", false},
		{"", "#ff0000", false},
	} {
		if got := ValidTokenIcon(c.icon) && ValidTokenColor(c.color); got != c.want {
			t.Errorf("icon %q colour %q: valid=%v, want %v", c.icon, c.color, got, c.want)
		}
	}
}
