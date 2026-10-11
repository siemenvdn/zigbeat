package measure

import (
	"sort"
	"time"
)

// Stats summarises the latency of answered samples and counts the lost ones.
type Stats struct {
	N, Lost       int
	P50, P95, Max time.Duration
}

// StatsOf summarises ss.
func StatsOf(ss []Sample) Stats {
	var st Stats
	var ls []time.Duration
	for _, s := range ss {
		if s.OK {
			ls = append(ls, s.Latency)
		} else {
			st.Lost++
		}
	}
	st.N = len(ls)
	if len(ls) == 0 {
		return st
	}
	sort.Slice(ls, func(i, j int) bool { return ls[i] < ls[j] })
	st.P50 = pct(ls, 0.50)
	st.P95 = pct(ls, 0.95)
	st.Max = ls[len(ls)-1]
	return st
}

func pct(sorted []time.Duration, p float64) time.Duration {
	i := int(float64(len(sorted)-1) * p)
	return sorted[i]
}

// Drifting compares the median latency of the first and last third of a step.
func Drifting(cmds []Sample) bool {
	var ok []Sample
	for _, s := range cmds {
		if s.OK {
			ok = append(ok, s)
		}
	}
	if len(ok) < 9 {
		return false
	}
	sort.Slice(ok, func(i, j int) bool { return ok[i].Sent.Before(ok[j].Sent) })
	third := len(ok) / 3
	first := StatsOf(ok[:third]).P50
	last := StatsOf(ok[len(ok)-third:]).P50
	return last > 2*first && last-first > 200*time.Millisecond
}
