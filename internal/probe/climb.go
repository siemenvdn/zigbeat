package probe

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/probe/measure"
	"github.com/siemenvdn/zigbeat/internal/probe/report"
)

const (
	// maxRate caps the climb after the -rates list, far above what Zigbee handles.
	maxRate = 100
	// climbFactor is the step after the -rates list: fine enough to find the knee.
	climbFactor = 1.25
	minClimb    = 1 // cmd/s, so that low rates still climb after rounding
	// recoveryWait is how long a stopped probe waits for the canary to settle; a
	// busy mesh empties its queues within seconds.
	recoveryWait = 60 * time.Second
	recoveryPoll = time.Second // about one canary read
	// settledReads is how many of the latest canary answers must be within the limit.
	settledReads = 3
)

// climbPercent is climbFactor as printed: "+25%".
func climbPercent() float64 { return (climbFactor - 1) * 100 }

// climb runs rate steps until the first sign of trouble and returns every step.
func (s *session) climb(lim measure.Limits) []measure.Step {
	var results []measure.Step
	report.Header(os.Stdout)
	rate, warned := s.cfg.rates[0], false
	for i := 0; rate <= maxRate; {
		if s.interrupted() || s.connLost() {
			break
		}
		res, ok := s.runStep(rate, lim)
		results = append(results, res)
		report.Row(os.Stdout, res)
		if !ok {
			break
		}
		if res.Verdict == "STOP" || res.Verdict == "WARN" && warned {
			s.waitRecovery(lim)
			break
		}
		// One warning can be noise: measure the same rate again before going higher.
		if warned = res.Verdict == "WARN"; !warned {
			i++
			rate = nextRate(s.cfg.rates, i, rate)
		}
		s.ph.Set("cooldown", res.Rate)
		if !s.sleep(s.cfg.cooldown) {
			break
		}
	}
	return results
}

// nextRate follows the -rates list, then climbs by climbFactor per step.
func nextRate(rates []float64, i int, prev float64) float64 {
	if i < len(rates) {
		return rates[i]
	}
	return math.Max(prev+minClimb, math.Round(prev*climbFactor))
}

// waitRecovery waits up to recoveryWait until the canary is back within its limit.
func (s *session) waitRecovery(lim measure.Limits) {
	s.ph.Set("recovery", 0)
	fmt.Println("stopping here, waiting for the network to settle")
	deadline := time.Now().Add(recoveryWait)
	for time.Now().Before(deadline) {
		if !s.sleep(recoveryPoll) {
			return
		}
		recent := s.canary.RecentOK(settledReads)
		if len(recent) < settledReads {
			continue
		}
		settled := true
		for _, d := range recent {
			if d > lim.CanaryLimit() {
				settled = false
			}
		}
		if settled {
			fmt.Println("network settled")
			return
		}
	}
	fmt.Printf("network still slow after %s; check the Zigbee2MQTT logs\n", cli.Seconds(recoveryWait))
}
