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

// Millis prints d in whole milliseconds, or - for none.
func Millis(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	return strconv.FormatInt(d.Milliseconds(), 10)
}

// Header prints the column titles of the step table.
func Header(w io.Writer) {
	fmt.Fprintf(w, "\n%-7s %5s %5s %5s %8s %8s %8s %6s %10s %10s %7s  %s\n",
		"rate", "sent", "ok", "lost", "p50 ms", "p95 ms", "max ms", "drift", "canary p95", "canary lost", "z2m err", "verdict")
}

// Row prints one step.
func Row(w io.Writer, r Step) {
	drift := "no"
	if r.Drift {
		drift = "yes"
	}
	fmt.Fprintf(w, "%-7s %5d %5d %5d %8s %8s %8s %6s %10s %10d %7d  %s",
		fmt.Sprintf("%g/s", r.Rate), r.Sent, r.Cmd.N, r.Cmd.Lost, Millis(r.Cmd.P50), Millis(r.Cmd.P95), Millis(r.Cmd.Max),
		drift, Millis(r.Canary.P95), r.Canary.Lost, r.Errs, r.Verdict)
	if len(r.Reasons) > 0 {
		fmt.Fprintf(w, " (%s)", strings.Join(r.Reasons, "; "))
	}
	fmt.Fprintln(w)
}

// Summary prints the highest clean rate and the budget it suggests.
func Summary(w io.Writer, results []Step, base Stats, canary string) {
	fmt.Fprintln(w, "\n== Summary ==")
	fmt.Fprintf(w, "Canary %s at rest: p50 %s ms, p95 %s ms\n", canary, Millis(base.P50), Millis(base.P95))
	b, ok := Suggest(results)
	if !ok {
		fmt.Fprintln(w, "No step ran clean. Even the lowest rate disturbed the network or lost commands.")
		return
	}
	fmt.Fprintf(w, "Highest clean rate: %g cmd/s\n", b.Clean.Rate)
	switch {
	case b.Interrupted:
		fmt.Fprintln(w, "  (run interrupted before the ceiling was reached)")
	case b.AllClean:
		fmt.Fprintln(w, "  (every step was clean, the real ceiling is higher than tested)")
	}
	fmt.Fprintf(w, "Suggested sustained budget: %g cmd/s (the clean rate / %d)\n", b.Rate, Headroom)
	fmt.Fprintf(w, "Command latency around that load (%g/s): p50 %s ms, p95 %s ms, jitter %s ms\n",
		b.Ref.Rate, Millis(b.Ref.Cmd.P50), Millis(b.Ref.Cmd.P95), Millis(b.Ref.Cmd.P95-b.Ref.Cmd.P50))
}

// WriteCSV writes every sample and log line to path.
func WriteCSV(path string, samples []Sample, logs []LogEvent) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"sent", "phase", "rate", "kind", "device", "latency_ms", "result"})
	for _, s := range samples {
		res := "ok"
		lat := strconv.FormatFloat(float64(s.Latency.Microseconds())/1000, 'f', 1, 64)
		if !s.OK {
			res, lat = "timeout", ""
		}
		_ = w.Write([]string{s.Sent.Format(time.RFC3339Nano), s.Phase, strconv.FormatFloat(s.Rate, 'g', -1, 64), s.Kind, s.Device, lat, res})
	}
	for _, l := range logs {
		_ = w.Write([]string{l.At.Format(time.RFC3339Nano), "z2m-log", "", l.Level, "", "", l.Msg})
	}
	w.Flush()
	return w.Error()
}
