package probe

import (
	"context"
	"math/rand"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

const (
	// pollEvery is far below the latencies measured, and cheap.
	pollEvery = 20 * time.Millisecond
	// drainGrace is more than expireEvery, so the expiry runs once more past -timeout.
	drainGrace = 300 * time.Millisecond
)

// sleep waits for d and returns false when interrupted or the connection drops.
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

// drain waits until every command is answered, then counts the rest as lost.
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
				s.canary.Tick(now, s.ph, func() error { return s.Get(s.cfg.canary, z2m.StateQuery) })
			}
		}
	}()
	go every(bg, expireEvery, s.tracker.Expire)
}
