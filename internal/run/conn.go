package run

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

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

// conn lets the lights package read and restore through the runner.
type conn struct{ r *runner }

func (c conn) Get(light string, p any) error { return c.r.link.Get(light, p) }
func (c conn) Set(light string, p any) error { return c.r.link.Set(light, p) }
func (c conn) Online() bool                  { return c.r.online() }

func (c conn) Await(ctx context.Context, light string, since time.Time, d time.Duration) (z2m.State, bool) {
	st, _, ok := c.r.waitState(ctx, light, since, d)
	return st, ok
}
