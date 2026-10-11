package probe

import (
	"time"

	"github.com/siemenvdn/zigbeat/internal/probe/measure"
)

// runStep sends commands at rate for one step and evaluates the result.
// It returns false when the step was interrupted.
func (s *session) runStep(rate float64, lim measure.Limits) (measure.Step, bool) {
	res := measure.Step{Rate: rate}
	s.ph.Set("step", rate)
	start := time.Now()
	completed := s.send(&res, start)
	s.ph.Set("drain", rate)
	s.drain()
	s.judge(&res, start, time.Now(), lim)
	if !completed && res.Verdict != "STOP" {
		res.Verdict = "PARTIAL"
		res.Reasons = append(res.Reasons, "step interrupted")
	}
	return res, completed
}

// send paces commands at res.Rate for one step from start. It returns false when interrupted.
func (s *session) send(res *measure.Step, start time.Time) bool {
	deadline := start.Add(s.cfg.step)
	interval := time.Duration(float64(time.Second) / res.Rate)
	turn := 0
	for next := start; next.Before(deadline); next = next.Add(interval) {
		if !s.sleep(time.Until(next)) {
			return false
		}
		// At most one command in flight per bulb: take the next target that is free.
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
		s.tracker.Add(dev, cmd.Key, measure.Pending{Sent: time.Now(), Phase: "step", Rate: res.Rate})
		_ = s.bridge.Set(dev, cmd.Payload)
		res.Sent++
	}
	return true
}

// judge collects the samples and log lines of the step that ran from start to end, and evaluates them.
func (s *session) judge(res *measure.Step, start, end time.Time, lim measure.Limits) {
	samples, logs := s.rec.Snapshot()
	var cmds, canary []measure.Sample
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
	res.Cmd = measure.StatsOf(cmds)
	res.Canary = measure.StatsOf(canary)
	measure.Evaluate(res, cmds, lim)
}
