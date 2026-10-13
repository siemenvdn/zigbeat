package probe

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/probe/palette"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

const (
	checkLevel = 200

	warmUp = 2 * time.Second
)

func (s *session) canaryAnswers() bool {
	if _, ok := s.Read(s.ctx, s.cfg.canary, s.cfg.timeout); ok {
		return true
	}
	if s.ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "canary %s does not answer; pick another device with -canary\n", s.cfg.canary)
	}
	return false
}

func (s *session) baseline() (Stats, bool) {
	s.ph.Set("baseline", 0)
	fmt.Printf("measuring baseline for %s\n", s.cfg.baseline)
	if !s.sleep(s.cfg.baseline) {
		return Stats{}, false
	}
	samples, _ := s.rec.Snapshot()
	var base []Sample
	for _, x := range samples {
		if x.Kind == "canary" && x.Phase == "baseline" {
			base = append(base, x)
		}
	}
	return StatsOf(base), true
}

func (s *session) selfCheck() bool {
	s.ph.Set("prep", 0)
	for _, t := range s.cfg.targets {
		_ = s.Set(t, map[string]any{"state": "ON", "brightness": checkLevel, "transition": 0})
	}
	s.sleep(warmUp)

	s.ph.Set("selfcheck", 0)
	for _, t := range s.cfg.targets {
		cmd := s.gen.Next(t)
		s.tracker.Add(t, cmd.Key, Pending{Sent: time.Now(), Phase: "selfcheck"})
		_ = s.Set(t, cmd.Payload)
	}
	s.drain()
	samples, _ := s.rec.Snapshot()
	matched := 0
	for _, x := range samples {
		if x.Phase == "selfcheck" && x.OK {
			matched++
		}
	}
	if matched == len(s.cfg.targets) {
		fmt.Printf("self-check ok, all %d targets answer\n", matched)
		return true
	}
	fmt.Fprintf(os.Stderr, "self-check failed: only %d of %d targets echoed a recognisable state\n", matched, len(s.cfg.targets))
	for _, t := range s.cfg.targets {
		st, _, _ := s.Last(t)
		b, _ := json.Marshal(st)
		fmt.Fprintf(os.Stderr, "  last state of %s: %s\n", t, b)
	}
	fmt.Fprintln(os.Stderr, "try again with -mode brightness")
	return false
}

func (s *session) refuseTargets() error {
	devs, err := s.Devices(s.ctx)
	if err != nil {
		if s.ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "warning: %v; the targets were not checked against it\n", err)
		}
		return nil
	}
	return checkTargets(devs, s.cfg.targets, s.cfg.mode)
}

func checkTargets(devs []z2m.Device, targets []string, mode palette.Mode) error {
	role := z2m.Color
	if mode == palette.Brightness {
		role = z2m.Breathe
	}
	for _, t := range targets {
		if err := z2m.Check(devs, t, role); err != nil {
			return err
		}
	}
	return nil
}
