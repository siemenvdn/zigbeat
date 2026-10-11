package probe

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// canaryAnswers checks that the canary replies to a state request.
func (s *session) canaryAnswers() bool {
	_ = s.bridge.Get(s.cfg.canary, map[string]string{"state": ""})
	select {
	case <-s.canaryAnswered:
		return true
	case <-time.After(5 * time.Second):
		fmt.Fprintf(os.Stderr, "canary %s does not answer; pick another device with -canary\n", s.cfg.canary)
		return false
	}
}

// baseline measures the canary alone, with nothing else from us on the air.
// It returns false when interrupted.
func (s *session) baseline() (latencyStats, bool) {
	s.ph.set("baseline", 0)
	fmt.Printf("measuring baseline for %s\n", s.cfg.baseline)
	if !s.sleep(s.cfg.baseline) {
		return latencyStats{}, false
	}
	samples, _ := s.rec.snapshot()
	var base []sample
	for _, x := range samples {
		if x.kind == "canary" && x.phase == "baseline" {
			base = append(base, x)
		}
	}
	return statsOf(base), true
}

// selfCheck turns the targets on and checks that each one echoes a command we can match.
func (s *session) selfCheck() bool {
	s.ph.set("prep", 0)
	for _, t := range s.cfg.targets {
		_ = s.bridge.Set(t, map[string]any{"state": "ON", "brightness": 200, "transition": 0})
	}
	s.sleep(2 * time.Second)

	s.ph.set("selfcheck", 0)
	for _, t := range s.cfg.targets {
		cmd := s.gen.nextFor(t)
		s.tracker.add(t, cmd.key, pending{sent: time.Now(), phase: "selfcheck"})
		_ = s.bridge.Set(t, cmd.payload)
	}
	s.drain()
	samples, _ := s.rec.snapshot()
	matched := 0
	for _, x := range samples {
		if x.phase == "selfcheck" && x.ok {
			matched++
		}
	}
	if matched == len(s.cfg.targets) {
		fmt.Printf("self-check ok, all %d targets answer\n", matched)
		return true
	}
	fmt.Fprintf(os.Stderr, "self-check failed: only %d of %d targets echoed a recognisable state\n", matched, len(s.cfg.targets))
	for _, t := range s.cfg.targets {
		s.mu.Lock()
		b, _ := json.Marshal(s.last[t].s)
		s.mu.Unlock()
		fmt.Fprintf(os.Stderr, "  last state of %s: %s\n", t, b)
	}
	fmt.Fprintln(os.Stderr, "try again with -mode brightness")
	return false
}
