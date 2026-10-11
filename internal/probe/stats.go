package probe

import (
	"sort"
	"time"
)

type latencyStats struct {
	n, lost       int
	p50, p95, max time.Duration
}

func statsOf(ss []sample) latencyStats {
	var st latencyStats
	var ls []time.Duration
	for _, s := range ss {
		if s.ok {
			ls = append(ls, s.latency)
		} else {
			st.lost++
		}
	}
	st.n = len(ls)
	if len(ls) == 0 {
		return st
	}
	sort.Slice(ls, func(i, j int) bool { return ls[i] < ls[j] })
	st.p50 = pct(ls, 0.50)
	st.p95 = pct(ls, 0.95)
	st.max = ls[len(ls)-1]
	return st
}

func pct(sorted []time.Duration, p float64) time.Duration {
	i := int(float64(len(sorted)-1) * p)
	return sorted[i]
}

// drifting compares the median latency of the first and last third of a step.
func drifting(cmds []sample) bool {
	var ok []sample
	for _, s := range cmds {
		if s.ok {
			ok = append(ok, s)
		}
	}
	if len(ok) < 9 {
		return false
	}
	sort.Slice(ok, func(i, j int) bool { return ok[i].sent.Before(ok[j].sent) })
	third := len(ok) / 3
	first := statsOf(ok[:third]).p50
	last := statsOf(ok[len(ok)-third:]).p50
	return last > 2*first && last-first > 200*time.Millisecond
}
