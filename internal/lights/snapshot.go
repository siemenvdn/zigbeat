package lights

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Snapshot reads the state of each light, one at a time at rate reads per second,
// two tries of 3 s each. Lights that do not answer are left out. It stops early
// when ctx is done.
func Snapshot(ctx context.Context, c Conn, lights []string, rate float64) map[string]z2m.State {
	snaps := map[string]z2m.State{}
	for _, l := range lights {
		for attempt := 0; attempt < 2; attempt++ {
			sent := time.Now()
			_ = c.Get(l, query)
			st, ok := c.Await(ctx, l, sent, 3*time.Second)
			if !sleep(ctx, pace(1, rate)-time.Since(sent)) {
				return snaps
			}
			if ok {
				snaps[l] = st
				break
			}
		}
	}
	return snaps
}
