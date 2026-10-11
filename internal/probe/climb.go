package probe

import (
	"fmt"
	"math"
	"os"
	"time"
)

// maxRate caps the climb after the -rates list, far above what Zigbee handles.
const maxRate = 100

// climb runs rate steps until the first sign of trouble and returns every step.
func (s *session) climb(lim limits) []stepResult {
	var results []stepResult
	printHeader(os.Stdout)
	rate, warned := s.cfg.rates[0], false
	for i := 0; rate <= maxRate; {
		if s.interrupted() || s.connLost() {
			break
		}
		res, ok := s.runStep(rate, lim)
		results = append(results, res)
		printRow(os.Stdout, res)
		if !ok {
			break
		}
		if res.verdict == "STOP" || res.verdict == "WARN" && warned {
			s.waitRecovery(lim)
			break
		}
		// One warning can be noise: measure the same rate again before going higher.
		if warned = res.verdict == "WARN"; !warned {
			i++
			rate = nextRate(s.cfg.rates, i, rate)
		}
		s.ph.set("cooldown", res.rate)
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

// runStep sends commands at rate for one step and evaluates the result.
// It returns false when the step was interrupted.
func (s *session) runStep(rate float64, lim limits) (stepResult, bool) {
	res := stepResult{rate: rate}
	s.ph.set("step", rate)
	start := time.Now()
	deadline := start.Add(s.cfg.step)
	interval := time.Duration(float64(time.Second) / rate)
	next := start
	completed := true
	turn := 0
	for ; next.Before(deadline); next = next.Add(interval) {
		if !s.sleep(time.Until(next)) {
			completed = false
			break
		}
		// At most one command in flight per bulb: take the next target that is free.
		dev := ""
		for j := range s.cfg.targets {
			if d := s.cfg.targets[(turn+j)%len(s.cfg.targets)]; !s.tracker.busy(d) {
				dev, turn = d, turn+j+1
				break
			}
		}
		if dev == "" {
			res.skipped++
			continue
		}
		cmd := s.gen.nextFor(dev)
		s.tracker.add(dev, cmd.key, pending{sent: time.Now(), phase: "step", rate: rate})
		_ = s.bridge.Set(dev, cmd.payload)
		res.sent++
	}

	s.ph.set("drain", rate)
	s.drain()
	end := time.Now()

	samples, logs := s.rec.snapshot()
	var cmds, canary []sample
	for _, x := range samples {
		if x.rate != rate || x.sent.Before(start) {
			continue
		}
		switch {
		case x.kind == "cmd" && x.phase == "step":
			cmds = append(cmds, x)
		case x.kind == "canary" && (x.phase == "step" || x.phase == "drain"):
			canary = append(canary, x)
		}
	}
	for _, l := range logs {
		if l.at.Before(start) || l.at.After(end) {
			continue
		}
		if l.level == "error" {
			res.errs++
		} else {
			res.warns++
		}
	}
	res.cmd = statsOf(cmds)
	res.canary = statsOf(canary)
	evaluate(&res, cmds, lim)
	if !completed && res.verdict != "STOP" {
		res.verdict = "PARTIAL"
		res.reasons = append(res.reasons, "step interrupted")
	}
	return res, completed
}

// waitRecovery waits up to 60 s until the canary is back within its limit.
func (s *session) waitRecovery(lim limits) {
	s.ph.set("recovery", 0)
	fmt.Println("stopping here, waiting for the network to settle")
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if !s.sleep(time.Second) {
			return
		}
		recent := s.canary.recentOK(3)
		if len(recent) < 3 {
			continue
		}
		settled := true
		for _, d := range recent {
			if d > lim.canaryLimit() {
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
