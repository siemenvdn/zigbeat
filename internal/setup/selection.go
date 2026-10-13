package setup

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const exampleRange = 3

func parseSelection(answer string, n int) ([]int, error) {
	bad := fmt.Errorf("answer with numbers from the list, such as 1-%d", min(n, exampleRange))
	var out []int
	for _, part := range strings.Split(answer, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		from, to, isRange := strings.Cut(part, "-")
		a, err := strconv.Atoi(strings.TrimSpace(from))
		if err != nil {
			return nil, bad
		}
		b := a
		if isRange {
			if b, err = strconv.Atoi(strings.TrimSpace(to)); err != nil {
				return nil, bad
			}
		}
		if a < 1 || b > n || a > b {
			return nil, bad
		}
		for i := a; i <= b; i++ {
			out = append(out, i-1)
		}
	}
	if len(out) == 0 {
		return nil, bad
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}
