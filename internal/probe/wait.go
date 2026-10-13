package probe

import (
	"context"
	"time"
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
