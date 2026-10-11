package probe

import (
	"fmt"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

type stepResult struct {
	rate    float64
	sent    int
	skipped int // ticks where every target still had a command in flight
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
	return z2m.CanaryLimit(l.canaryBaseP95)
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
	if r.skipped*10 > r.sent+r.skipped {
		warn(fmt.Sprintf("targets still busy, sent %d of %d; more -targets may go higher", r.sent, r.sent+r.skipped))
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
