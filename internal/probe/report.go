package probe

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

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
