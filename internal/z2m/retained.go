package z2m

import (
	"context"
	"time"
)

// RetainedWait is how long a command waits for a retained bridge message after
// subscribing. The broker sends it right after the SUBACK; the margin is for a
// bridge/devices of 1.5 MB from a busy Raspberry Pi.
const RetainedWait = 5 * time.Second

// Groups waits up to RetainedWait for the retained list of groups; false when none came.
func (b *Bridge) Groups(ctx context.Context) ([]string, bool) { return first(ctx, b.OnGroups) }

// Devices waits up to RetainedWait for the retained device list; false when none came.
func (b *Bridge) Devices(ctx context.Context) ([]Device, bool) { return first(ctx, b.OnDevices) }

// first subscribes with on and returns the first message, or false after
// RetainedWait or when ctx is done.
func first[T any](ctx context.Context, on func(func(T, time.Time)) error) (T, bool) {
	var none T
	got := make(chan T, 1)
	err := on(func(v T, _ time.Time) {
		select {
		case got <- v:
		default:
		}
	})
	if err != nil {
		return none, false
	}
	t := time.NewTimer(RetainedWait)
	defer t.Stop()
	select {
	case v := <-got:
		return v, true
	case <-t.C:
	case <-ctx.Done():
	}
	return none, false
}
