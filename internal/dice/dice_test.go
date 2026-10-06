package dice

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestRoll(t *testing.T) {
	cases := []struct {
		expr    string
		wantErr bool
	}{
		{"2d6+3", false},
		{"1d20", false},
		{"2d6+1d4-2", false},
		{"5", false},
		{"", true},
		{"2d6x3", true},
		{"d20", false},
		{"0d6", true},
		{"1d0", true},
		{"101d6", true},
	}

	for _, c := range cases {
		result, err := Roll(c.expr)
		if c.wantErr {
			if err == nil {
				t.Errorf("Roll(%q): expected error, got result %+v", c.expr, result)
			}
			continue
		}
		if err != nil {
			t.Errorf("Roll(%q): unexpected error: %v", c.expr, err)
			continue
		}
		if result.Breakdown == "" {
			t.Errorf("Roll(%q): empty breakdown", c.expr)
		}
	}
}

var breakdownTerm = regexp.MustCompile(`([+-]?)(\d+)d(\d+)\(([\d,]+)\)`)

// Sorted so that counting successes stops at the first miss. Rolled many
// times over a big pool, because one roll can come out in order by luck —
// and with a d20 between two pools of d6, because each term is sorted on
// its own and a d20 sorted in among d6s would mean nothing. The sum check
// is what proves the sorted list is still the dice that made the total,
// rather than some other list that happens to be in order.
func TestRoll_ListsEachTermsDiceHighestFirst(t *testing.T) {
	for i := 0; i < 50; i++ {
		result, err := Roll("40d6+1d20-30d6+3")
		if err != nil {
			t.Fatalf("Roll: %v", err)
		}
		terms := breakdownTerm.FindAllStringSubmatch(result.Breakdown, -1)
		if len(terms) != 3 {
			t.Fatalf("breakdown %q: want 3 dice terms, got %d", result.Breakdown, len(terms))
		}
		sum := 3
		for _, term := range terms {
			fields := strings.Split(term[4], ",")
			if want, _ := strconv.Atoi(term[2]); len(fields) != want {
				t.Fatalf("term %q: want %d dice, got %d", term[0], want, len(fields))
			}
			prev := 0
			for j, f := range fields {
				v, _ := strconv.Atoi(f)
				if j > 0 && v > prev {
					t.Fatalf("term %q is not highest first", term[0])
				}
				prev = v
				if term[1] == "-" {
					sum -= v
				} else {
					sum += v
				}
			}
		}
		if sum != result.Total {
			t.Fatalf("breakdown %q sums to %d, total says %d", result.Breakdown, sum, result.Total)
		}
	}
}

func TestRollBounds(t *testing.T) {
	for i := 0; i < 200; i++ {
		result, err := Roll("2d6")
		if err != nil {
			t.Fatalf("Roll(2d6): %v", err)
		}
		if result.Total < 2 || result.Total > 12 {
			t.Fatalf("Roll(2d6) out of bounds: %d", result.Total)
		}
	}
}
