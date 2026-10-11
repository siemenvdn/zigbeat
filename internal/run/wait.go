package run

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// pace keeps one-at-a-time phases under the budget: frames / max_rate.
func (r *runner) pace(frames int) time.Duration {
	return time.Duration(float64(frames) / r.cfg.Budget.MaxRate * float64(time.Second))
}

// sleep waits d, or less when ctx is done; it reports whether ctx is still live.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(max(0, d))
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// waitState reads events until light reports a state after since, or d passes.
// Other events are dropped: the show is not running.
func (r *runner) waitState(ctx context.Context, light string, since time.Time, d time.Duration) (z2m.State, time.Time, bool) {
	deadline := time.After(d)
	for {
		select {
		case ev := <-r.events:
			if ev.light == light && ev.at.After(since) {
				return ev.st, ev.at, true
			}
		case <-deadline:
			return z2m.State{}, time.Time{}, false
		case <-r.link.Lost():
			return z2m.State{}, time.Time{}, false
		case <-ctx.Done():
			return z2m.State{}, time.Time{}, false
		}
	}
}
