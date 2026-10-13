package probe

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
)

const (
	maxRate = 100

	climbFactor = 1.25
	minClimb    = 1

	recoveryWait = 60 * time.Second
	recoveryPoll = time.Second

	settledReads = 3
)

func climbPercent() float64 { return (climbFactor - 1) * 100 }

func (s *session) climb(lim Limits) []Step {
	var results []Step
	Header(os.Stdout)
	rate, warned := s.cfg.rates[0], false
	for i := 0; rate <= maxRate; {
		if s.interrupted() || s.connLost() {
			break
		}
		res, ok := s.runStep(rate, lim)
		results = append(results, res)
		Row(os.Stdout, res)
		if !ok {
			break
		}
		if res.Verdict == Stop || res.Verdict == Warn && warned {
			s.waitRecovery(lim)
			break
		}

		if warned = res.Verdict == Warn; !warned {
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

func nextRate(rates []float64, i int, prev float64) float64 {
	if i < len(rates) {
		return rates[i]
	}
	return math.Max(prev+minClimb, math.Round(prev*climbFactor))
}

func (s *session) waitRecovery(lim Limits) {
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

func (s *session) runStep(rate float64, lim Limits) (Step, bool) {
	res := Step{Rate: rate}
	s.ph.Set("step", rate)
	start := time.Now()
	completed := s.send(&res, start)
	s.ph.Set("drain", rate)
	s.drain()
	s.judge(&res, start, time.Now(), lim)
	if !completed && res.Verdict != Stop {
		res.Verdict = Partial
		res.Reasons = append(res.Reasons, "step interrupted")
	}
	return res, completed
}

func (s *session) send(res *Step, start time.Time) bool {
	deadline := start.Add(s.cfg.step)
	interval := time.Duration(float64(time.Second) / res.Rate)
	turn := 0
	for next := start; next.Before(deadline); next = next.Add(interval) {
		if !s.sleep(time.Until(next)) {
			return false
		}

		dev := ""
		for j := range s.cfg.targets {
			if d := s.cfg.targets[(turn+j)%len(s.cfg.targets)]; !s.tracker.Busy(d) {
				dev, turn = d, turn+j+1
				break
			}
		}
		if dev == "" {
			res.Skipped++
			continue
		}
		cmd := s.gen.Next(dev)
		s.tracker.Add(dev, cmd.Key, Pending{Sent: time.Now(), Phase: "step", Rate: res.Rate})
		_ = s.Set(dev, cmd.Payload)
		res.Sent++
	}
	return true
}

func (s *session) judge(res *Step, start, end time.Time, lim Limits) {
	samples, logs := s.rec.Snapshot()
	var cmds, canary []Sample
	for _, x := range samples {
		if x.Rate != res.Rate || x.Sent.Before(start) {
			continue
		}
		switch {
		case x.Kind == "cmd" && x.Phase == "step":
			cmds = append(cmds, x)
		case x.Kind == "canary" && (x.Phase == "step" || x.Phase == "drain"):
			canary = append(canary, x)
		}
	}
	for _, l := range logs {
		if l.At.Before(start) || l.At.After(end) {
			continue
		}
		if l.Level == "error" {
			res.Errs++
		} else {
			res.Warns++
		}
	}
	res.Cmd = StatsOf(cmds)
	res.Canary = StatsOf(canary)
	Evaluate(res, cmds, lim)
}
