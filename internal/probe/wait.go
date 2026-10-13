package probe

import (
	"context"
	"math/rand"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

const (
	pollEvery = 20 * time.Millisecond

	drainGrace = 300 * time.Millisecond
)

func (s *session) sleep(d time.Duration) bool {
	if d <= 0 {
		select {
		case <-s.ctx.Done():
			return false
		case <-s.Done():
			return false
		default:
			return true
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-s.ctx.Done():
		return false
	case <-s.Done():
		return false
	}
}

func (s *session) drain() {
	waitFor(s.cfg.timeout+drainGrace, func() bool { return s.tracker.InFlight() == 0 })
	s.tracker.Expire(time.Now().Add(s.cfg.timeout))
}

func waitFor(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for {
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(pollEvery)
	}
}

func every(ctx context.Context, d time.Duration, fn func(time.Time)) {
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case now := <-t.C:
			fn(now)
		case <-ctx.Done():
			return
		}
	}
}

const (
	canaryMin    = 700 * time.Millisecond
	canarySpread = 600 * time.Millisecond

	expireEvery = 200 * time.Millisecond
)

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
				s.canary.Tick(now, s.ph, func() error { return s.Get(s.cfg.canary, z2m.StateQuery) })
			}
		}
	}()
	go every(bg, expireEvery, s.tracker.Expire)
}
