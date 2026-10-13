package probe

import (
	"testing"
)

func steps(verdicts ...Verdict) []Step {
	rates := []float64{1, 2, 4, 8, 12, 15, 19, 24, 30, 38}
	var out []Step
	for i, v := range verdicts {
		out = append(out, Step{Rate: rates[i], Verdict: v})
	}
	return out
}

func TestSuggest(t *testing.T) {
	cases := []struct {
		name     string
		results  []Step
		ok       bool
		clean    float64
		rate     float64
		ref      float64
		partial  bool
		allClean bool
	}{
		{"stop", steps("OK", "OK", "OK", "OK", "OK", "STOP"), true, 12, 6, 4, false, false},
		{"warn then stop", steps("OK", "OK", "OK", "WARN", "STOP"), true, 4, 2, 2, false, false},
		{"all clean", steps("OK", "OK", "OK"), true, 4, 2, 2, false, true},
		{"interrupted", steps("OK", "OK", "OK", "OK", "PARTIAL"), true, 8, 4, 4, true, false},
		{"none clean", steps("WARN", "STOP"), false, 0, 0, 0, false, false},
		{"empty", nil, false, 0, 0, 0, false, false},
	}
	for _, c := range cases {
		b, ok := Suggest(c.results)
		if ok != c.ok || b.Clean.Rate != c.clean || b.Rate != c.rate || b.Ref.Rate != c.ref || b.Interrupted != c.partial || b.AllClean != c.allClean {
			t.Errorf("%s: %+v, %v", c.name, b, ok)
		}
	}
}
