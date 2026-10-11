package probe

import (
	"context"
	"math/rand"
	"time"
)

// startBackground starts the canary probes and the expiry of unanswered commands.
func (s *session) startBackground() {
	bg, stop := context.WithCancel(context.Background())
	s.stopBg = stop
	// Irregular probe interval so probes never lock in phase with the command rate.
	go func() {
		for {
			d := 700*time.Millisecond + time.Duration(rand.Int63n(int64(600*time.Millisecond)))
			select {
			case <-bg.Done():
				return
			case now := <-time.After(d):
				s.canary.Tick(now, s.ph, func() error { return s.bridge.Get(s.cfg.canary, map[string]string{"state": ""}) })
			}
		}
	}()
	go every(bg, 200*time.Millisecond, s.tracker.Expire)
}
