// Package report prints the probe's results and writes its raw samples.
package report

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/siemenvdn/zigbeat/internal/probe/measure"
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
func Row(w io.Writer, r measure.Step) {
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
func Summary(w io.Writer, results []measure.Step, base measure.Stats, canary string) {
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
