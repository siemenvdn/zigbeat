package probe

import (
	"fmt"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Step is the outcome of one rate step.
type Step struct {
	Rate    float64
	Sent    int
	Skipped int
	Cmd     Stats
	Drift   bool
	Canary  Stats
	Errs    int
	Warns   int
	Verdict Verdict
	Reasons []string
}

// Verdict is how a step went.
type Verdict string

// The verdicts.
const (
	OK      Verdict = "OK"
	Warn    Verdict = "WARN"
	Stop    Verdict = "STOP"
	Partial Verdict = "PARTIAL"
)

// Limits come from the canary at rest.
type Limits struct {
	CanaryP50 time.Duration
	CanaryP95 time.Duration
}

// CanaryLimit is the canary latency above which a step fails.
func (l Limits) CanaryLimit() time.Duration {
	return z2m.CanaryLimit(l.CanaryP95)
}

const (
	maxLost      = 0.05
	maxSkipped   = 0.1
	maxP95       = 1500 * time.Millisecond
	driftStopP95 = 800 * time.Millisecond
	canarySlower = 100 * time.Millisecond

	canaryLostStop = 2

	errStop = 3
)

// Evaluate sets the verdict of r and the reasons for it.
func Evaluate(r *Step, cmds []Sample, l Limits) {
	r.Drift = Drifting(cmds)
	total := r.Cmd.N + r.Cmd.Lost
	lostRatio := 0.0
	if total > 0 {
		lostRatio = float64(r.Cmd.Lost) / float64(total)
	}
	stop := func(s string) { r.Reasons = append(r.Reasons, s); r.Verdict = Stop }
	warn := func(s string) {
		r.Reasons = append(r.Reasons, s)
		if r.Verdict == OK {
			r.Verdict = Warn
		}
	}
	r.Verdict = OK
	switch {
	case lostRatio > maxLost:
		stop(fmt.Sprintf("%.0f%% of commands unanswered", lostRatio*100))
	case r.Cmd.Lost > 0:
		warn(fmt.Sprintf("%d commands unanswered", r.Cmd.Lost))
	}
	if float64(r.Skipped) > maxSkipped*float64(r.Sent+r.Skipped) {
		warn(fmt.Sprintf("targets still busy, sent %d of %d; more -targets may go higher", r.Sent, r.Sent+r.Skipped))
	}
	if r.Cmd.P95 > maxP95 {
		stop("command p95 above " + cli.Seconds(maxP95))
	}
	if r.Drift {
		if r.Cmd.P95 > driftStopP95 {
			stop("latency keeps growing (queue builds up)")
		} else {
			warn("latency trending up")
		}
	}
	if r.Canary.P95 > l.CanaryLimit() {
		stop("rest of the network slowed down (canary)")
	} else if r.Canary.P50 > l.CanaryP50+canarySlower {
		warn("rest of the network noticeably slower")
	}
	if r.Canary.Lost >= canaryLostStop {
		stop("canary stopped answering")
	} else if r.Canary.Lost == 1 {
		warn("one canary probe lost")
	}
	if r.Errs >= errStop {
		stop("Zigbee2MQTT logged errors")
	} else if r.Errs > 0 {
		warn("Zigbee2MQTT logged an error")
	}
}
