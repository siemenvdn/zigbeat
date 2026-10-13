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
	// checkLevel turns the targets on, clearly visible, before the self-check.
	checkLevel = 200
	// warmUp lets the echoes of turning on arrive, so the self-check does not
	// take them for its own answers.
	warmUp = 2 * time.Second
)

// canaryAnswers checks that the canary replies to a state request.
func (s *session) canaryAnswers() bool {
	if _, ok := s.Read(s.ctx, s.cfg.canary, s.cfg.timeout); ok {
		return true
	}
	fmt.Fprintf(os.Stderr, "canary %s does not answer; pick another device with -canary\n", s.cfg.canary)
	return false
}

// baseline measures the canary alone, with nothing else from us on the air.
// It returns false when interrupted.
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

// selfCheck turns the targets on and checks that each one echoes a command we can match.
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

// refuseTargets stops the probe before anything changes when a target may not
// be driven by the rules of zigbeat setup. Without bridge/devices it warns and goes on.
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

// checkTargets checks every target in the role its mode needs: brightness
// mode drives any light, color and combo need xy colour.
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
