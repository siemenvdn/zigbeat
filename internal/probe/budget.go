package probe

// Headroom divides the highest clean rate into the suggested budget: the probe
// ran minutes on a quiet network, a show runs hours next to the rest of the house.
const Headroom = 2

// Budget is what a probe run suggests.
type Budget struct {
	Clean       Step    // the highest step that ran clean before any STOP
	Rate        float64 // the suggested sustained rate in commands per second
	Ref         Step    // the highest step at or below Rate, for the latency at that load
	Interrupted bool    // the run ended before the ceiling was found
	AllClean    bool    // every step was clean: the ceiling is higher than tested
}

// Suggest finds the budget in the steps of a run; false when no step ran clean.
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
