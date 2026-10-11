package probe

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// saveState reads the current state of every target so it can be put back afterwards.
func (s *session) saveState() bool {
	at := time.Now()
	for _, t := range s.cfg.targets {
		_ = s.bridge.Get(t, z2m.SnapshotQuery)
	}
	if !waitFor(5*time.Second, func() bool {
		for _, t := range s.cfg.targets {
			st, ok := s.answeredSince(t, at)
			if !ok {
				return false
			}
			s.snaps[t] = st
		}
		return true
	}) {
		for _, t := range s.cfg.targets {
			if _, ok := s.snaps[t]; !ok {
				fmt.Fprintf(os.Stderr, "no answer from %s; check the friendly name and that it is powered\n", t)
			}
		}
		return false
	}
	s.snapJSON, _ = json.Marshal(s.snaps)
	return true
}

// restore puts every target back to its saved state, with one retry per target.
func (s *session) restore() {
	s.ph.set("restore", 0)
	fmt.Println("\nrestoring lights")
	failed := 0
	for _, t := range s.cfg.targets {
		ok := true
		for _, p := range z2m.RestorePayloads(s.snaps[t]) {
			ok = false
			for attempt := 0; attempt < 2 && !ok; attempt++ {
				sent := time.Now()
				_ = s.bridge.Set(t, p)
				ok = waitFor(3*time.Second, func() bool { _, ok := s.answeredSince(t, sent); return ok })
			}
			if !ok {
				break
			}
		}
		if !ok {
			failed++
			fmt.Printf("  could not confirm %s, set it back by hand\n", t)
		}
	}
	if failed == 0 {
		fmt.Println("  all targets back to their previous state")
	}
}
