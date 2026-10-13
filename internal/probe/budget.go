package probe

// Headroom divides the highest clean rate into the suggested budget.
const Headroom = 2

// Budget is what a probe run suggests.
type Budget struct {
	Clean       Step
	Rate        float64
	Ref         Step
	Interrupted bool
	AllClean    bool
}

// Suggest finds the budget in the steps of a run.
func Suggest(results []Step) (Budget, bool) {
	clean := -1
	for i, r := range results {
		if r.Verdict == Stop {
			break
		}
		if r.Verdict == OK {
			clean = i
		}
	}
	if clean < 0 {
		return Budget{}, false
	}
	b := Budget{
		Clean:       results[clean],
		Rate:        results[clean].Rate / Headroom,
		Ref:         results[clean],
		Interrupted: results[len(results)-1].Verdict == Partial,
		AllClean:    clean == len(results)-1,
	}
	for _, r := range results[:clean+1] {
		if r.Rate <= b.Rate {
			b.Ref = r
		}
	}
	return b, true
}
