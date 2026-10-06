// Package dice parses and evaluates dice expressions like "2d6+3" or
// "1d20+1d4-2".
package dice

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Result struct {
	Expression string
	Total      int
	Breakdown  string
}

const (
	maxDiceCount = 100
	maxDiceSides = 1000
)

var termPattern = regexp.MustCompile(`([+-]?)\s*(\d*d\d+|\d+)`)

// Roll evaluates a dice expression such as "2d6+3". Terms are separated
// by + or -; each term is either NdM (roll N dice with M sides, N
// defaults to 1) or a flat integer modifier.
func Roll(expr string) (Result, error) {
	trimmed := strings.ToLower(strings.TrimSpace(expr))
	if trimmed == "" {
		return Result{}, fmt.Errorf("empty expression")
	}

	matches := termPattern.FindAllStringSubmatchIndex(trimmed, -1)
	if matches == nil {
		return Result{}, fmt.Errorf("no valid dice terms in %q", expr)
	}
	if err := verifyFullyMatched(trimmed, matches); err != nil {
		return Result{}, err
	}

	total := 0
	var parts []string

	for _, m := range matches {
		sign := trimmed[m[2]:m[3]]
		term := trimmed[m[4]:m[5]]

		negative := sign == "-"

		if strings.Contains(term, "d") {
			count, sides, err := parseDice(term)
			if err != nil {
				return Result{}, err
			}
			rolls := make([]int, count)
			sum := 0
			for i := range rolls {
				rolls[i] = rand.IntN(sides) + 1
				sum += rolls[i]
			}
			if negative {
				sum = -sum
			}
			total += sum
			// Highest first, so counting how many dice beat a target stops
			// at the first miss rather than reading all of them — the
			// complaint from a twelve-die roll in playtesting. Sorted here
			// rather than in the client because the breakdown is stored as
			// text: the message is sorted for everyone, a refresh included,
			// and nothing has to parse numbers back out of a string. Per
			// term, since a d20 sorted in among d6s means nothing. The order
			// the dice fell in is dropped, and nothing at the table needs it.
			slices.Sort(rolls)
			slices.Reverse(rolls)
			parts = append(parts, fmt.Sprintf("%s%dd%d(%s)", signPrefix(sign), count, sides, joinInts(rolls)))
		} else {
			value, err := strconv.Atoi(term)
			if err != nil {
				return Result{}, fmt.Errorf("invalid modifier %q: %w", term, err)
			}
			if negative {
				value = -value
			}
			total += value
			parts = append(parts, fmt.Sprintf("%s%d", signPrefix(sign), value))
		}
	}

	return Result{
		Expression: expr,
		Total:      total,
		Breakdown:  strings.TrimPrefix(strings.Join(parts, ""), "+"),
	}, nil
}

// verifyFullyMatched rejects expressions with stray characters between
// or around recognized terms (e.g. "2d6x3"), rather than silently
// ignoring them.
func verifyFullyMatched(expr string, matches [][]int) error {
	pos := 0
	for _, m := range matches {
		if strings.TrimSpace(expr[pos:m[0]]) != "" {
			return fmt.Errorf("unrecognized characters in expression near %q", expr[pos:m[0]])
		}
		pos = m[1]
	}
	if strings.TrimSpace(expr[pos:]) != "" {
		return fmt.Errorf("unrecognized characters in expression near %q", expr[pos:])
	}
	return nil
}

func parseDice(term string) (count, sides int, err error) {
	parts := strings.SplitN(term, "d", 2)
	countStr, sidesStr := parts[0], parts[1]

	count = 1
	if countStr != "" {
		count, err = strconv.Atoi(countStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid dice count in %q: %w", term, err)
		}
	}
	sides, err = strconv.Atoi(sidesStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid dice sides in %q: %w", term, err)
	}

	if count < 1 || count > maxDiceCount {
		return 0, 0, fmt.Errorf("dice count must be between 1 and %d, got %d", maxDiceCount, count)
	}
	if sides < 1 || sides > maxDiceSides {
		return 0, 0, fmt.Errorf("dice sides must be between 1 and %d, got %d", maxDiceSides, sides)
	}

	return count, sides, nil
}

func signPrefix(sign string) string {
	if sign == "" {
		return "+"
	}
	return sign
}

func joinInts(vals []int) string {
	strs := make([]string, len(vals))
	for i, v := range vals {
		strs[i] = strconv.Itoa(v)
	}
	return strings.Join(strs, ",")
}
