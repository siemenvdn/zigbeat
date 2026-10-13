package probe

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/siemenvdn/zigbeat/internal/lights"
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
	s.ph.Set("restore", 0)
	fmt.Println()
	lights.Restore(s, s.snaps, s.cfg.targets, s.cfg.rates[0])
}

// Online lets the restore stop when the broker is gone, saying once what to restore by hand.
func (s *session) Online() bool { return !s.connLost() }
