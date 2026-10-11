package probe

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/probe/measure"
	"github.com/siemenvdn/zigbeat/internal/probe/report"
)

// maxRate caps the climb after the -rates list, far above what Zigbee handles.
const maxRate = 100

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

// nextRate follows the -rates list, then climbs by 25% per step.
func nextRate(rates []float64, i int, prev float64) float64 {
	if i < len(rates) {
		return rates[i]
	}
	return math.Max(prev+1, math.Round(prev*1.25))
}

// waitRecovery waits up to 60 s until the canary is back within its limit.
func (s *session) waitRecovery(lim measure.Limits) {
	s.ph.Set("recovery", 0)
	fmt.Println("stopping here, waiting for the network to settle")
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if !s.sleep(time.Second) {
			return
		}
		recent := s.canary.RecentOK(3)
		if len(recent) < 3 {
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
	fmt.Println("network still slow after 60 s; check the Zigbee2MQTT logs")
}
