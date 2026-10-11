package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/lights"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// saveState reads the current state of every target so it can be put back afterwards.
func (s *session) saveState() bool {
	s.snaps = lights.Snapshot(s.ctx, s, s.cfg.targets, s.cfg.rates[0])
	if s.ctx.Err() != nil {
		fmt.Println("cancelled, nothing was changed")
		return false
	}
	ok := true
	for _, t := range s.cfg.targets {
		if _, got := s.snaps[t]; !got {
			fmt.Fprintf(os.Stderr, "no answer from %s; check the friendly name and that it is powered\n", t)
			ok = false
		}
	}
	s.snapJSON, _ = json.Marshal(s.snaps)
	return ok
}

// restore puts every target back to its saved state.
func (s *session) restore() {
	s.ph.set("restore", 0)
	fmt.Println()
	lights.Restore(s, s.snaps, s.cfg.targets, s.cfg.rates[0])
}

// Get, Set, Await and Online let the lights package read and restore through the session.
func (s *session) Get(light string, p any) error { return s.bridge.Get(light, p) }
func (s *session) Set(light string, p any) error { return s.bridge.Set(light, p) }
func (s *session) Online() bool                  { return !s.connLost() }

func (s *session) Await(ctx context.Context, light string, since time.Time, d time.Duration) (z2m.State, bool) {
	deadline := time.Now().Add(d)
	for {
		if st, ok := s.answeredSince(light, since); ok {
			return st, true
		}
		// Not s.sleep: the restore runs after Ctrl-C, when s.ctx is already done.
		if time.Now().After(deadline) || ctx.Err() != nil || s.client.Closed() {
			return z2m.State{}, false
		}
		time.Sleep(20 * time.Millisecond)
	}
}
