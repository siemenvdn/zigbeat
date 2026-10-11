package probe

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
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

type stepResult struct {
	rate    float64
	sent    int
	cmd     latencyStats
	drift   bool
	canary  latencyStats
	errs    int
	warns   int
	verdict string
	reasons []string
}

type limits struct {
	canaryBaseP50 time.Duration
	canaryBaseP95 time.Duration
}

func (l limits) canaryLimit() time.Duration {
	a, b := 3*l.canaryBaseP95, l.canaryBaseP95+300*time.Millisecond
	if a > b {
		return a
	}
	return b
}

func evaluate(r *stepResult, cmds []sample, l limits) {
	r.drift = drifting(cmds)
	total := r.cmd.n + r.cmd.lost
	lostRatio := 0.0
	if total > 0 {
		lostRatio = float64(r.cmd.lost) / float64(total)
	}
	stop := func(s string) { r.reasons = append(r.reasons, s); r.verdict = "STOP" }
	warn := func(s string) {
		r.reasons = append(r.reasons, s)
		if r.verdict == "OK" {
			r.verdict = "WARN"
		}
	}
	r.verdict = "OK"
	switch {
	case lostRatio > 0.05:
		stop(fmt.Sprintf("%.0f%% of commands unanswered", lostRatio*100))
	case r.cmd.lost > 0:
		warn(fmt.Sprintf("%d commands unanswered", r.cmd.lost))
	}
	if r.cmd.p95 > 1500*time.Millisecond {
		stop("command p95 above 1.5 s")
	}
	if r.drift {
		if r.cmd.p95 > 800*time.Millisecond {
			stop("latency keeps growing (queue builds up)")
		} else {
			warn("latency trending up")
		}
	}
	if r.canary.p95 > l.canaryLimit() {
		stop("rest of the network slowed down (canary)")
	} else if r.canary.p50 > l.canaryBaseP50+100*time.Millisecond {
		warn("rest of the network noticeably slower")
	}
	if r.canary.lost >= 2 {
		stop("canary stopped answering")
	} else if r.canary.lost == 1 {
		warn("one canary probe lost")
	}
	if r.errs >= 3 {
		stop("Zigbee2MQTT logged errors")
	} else if r.errs > 0 {
		warn("Zigbee2MQTT logged an error")
	}
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

func ms(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	return strconv.FormatInt(d.Milliseconds(), 10)
}

func printHeader(w io.Writer) {
	fmt.Fprintf(w, "\n%-7s %5s %5s %5s %8s %8s %8s %6s %10s %10s %7s  %s\n",
		"rate", "sent", "ok", "lost", "p50 ms", "p95 ms", "max ms", "drift", "canary p95", "canary lost", "z2m err", "verdict")
}

func printRow(w io.Writer, r stepResult) {
	drift := "no"
	if r.drift {
		drift = "yes"
	}
	fmt.Fprintf(w, "%-7s %5d %5d %5d %8s %8s %8s %6s %10s %10d %7d  %s",
		fmt.Sprintf("%g/s", r.rate), r.sent, r.cmd.n, r.cmd.lost, ms(r.cmd.p50), ms(r.cmd.p95), ms(r.cmd.max),
		drift, ms(r.canary.p95), r.canary.lost, r.errs, r.verdict)
	if len(r.reasons) > 0 {
		fmt.Fprintf(w, " (%s)", strings.Join(r.reasons, "; "))
	}
	fmt.Fprintln(w)
}

func printSummary(w io.Writer, results []stepResult, base latencyStats, canary string) {
	fmt.Fprintln(w, "\n== Summary ==")
	fmt.Fprintf(w, "Canary %s at rest: p50 %s ms, p95 %s ms\n", canary, ms(base.p50), ms(base.p95))
	clean := -1
	for i, r := range results {
		if r.verdict == "STOP" {
			break
		}
		if r.verdict == "OK" {
			clean = i
		}
	}
	if clean < 0 {
		fmt.Fprintln(w, "No step ran clean. Even the lowest rate disturbed the network or lost commands.")
		return
	}
	top := results[clean]
	budget := top.rate / 2
	fmt.Fprintf(w, "Highest clean rate: %g cmd/s\n", top.rate)
	switch {
	case results[len(results)-1].verdict == "PARTIAL":
		fmt.Fprintln(w, "  (run interrupted before the ceiling was reached)")
	case clean == len(results)-1:
		fmt.Fprintln(w, "  (every step was clean, the real ceiling is higher than tested)")
	}
	fmt.Fprintf(w, "Suggested sustained budget: %g cmd/s (half the clean rate)\n", budget)
	ref := top
	for _, r := range results[:clean+1] {
		if r.rate <= budget {
			ref = r
		}
	}
	fmt.Fprintf(w, "Command latency around that load (%g/s): p50 %s ms, p95 %s ms, jitter %s ms\n",
		ref.rate, ms(ref.cmd.p50), ms(ref.cmd.p95), ms(ref.cmd.p95-ref.cmd.p50))
}

func writeCSV(path string, samples []sample, logs []logEvent) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"sent", "phase", "rate", "kind", "device", "latency_ms", "result"})
	for _, s := range samples {
		res := "ok"
		lat := strconv.FormatFloat(float64(s.latency.Microseconds())/1000, 'f', 1, 64)
		if !s.ok {
			res, lat = "timeout", ""
		}
		_ = w.Write([]string{s.sent.Format(time.RFC3339Nano), s.phase, strconv.FormatFloat(s.rate, 'g', -1, 64), s.kind, s.device, lat, res})
	}
	for _, l := range logs {
		_ = w.Write([]string{l.at.Format(time.RFC3339Nano), "z2m-log", "", l.level, "", "", l.msg})
	}
	w.Flush()
	return w.Error()
}
