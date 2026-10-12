package probe

import (
	"context"
	"math/rand"
	"time"
)

const (
	// The canary is read every canaryMin to canaryMin+canarySpread: about once a
	// second, and irregular so the reads never lock in phase with the command rate.
	canaryMin    = 700 * time.Millisecond
	canarySpread = 600 * time.Millisecond
	// expireEvery checks unanswered commands against -timeout, which is seconds.
	expireEvery = 200 * time.Millisecond
)

// startBackground starts the canary probes and the expiry of unanswered commands.
func (s *session) startBackground() {
	bg, stop := context.WithCancel(context.Background())
	s.stopBg = stop
	go func() {
		for {
			d := canaryMin + time.Duration(rand.Int63n(int64(canarySpread)))
			select {
			case <-bg.Done():
				return
			case now := <-time.After(d):
				s.canary.Tick(now, s.ph, func() error { return s.bridge.Get(s.cfg.canary, map[string]string{"state": ""}) })
			}
		}
	}()
	go every(bg, expireEvery, s.tracker.Expire)
}
