package dice

import "testing"

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
