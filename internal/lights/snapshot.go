package lights

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// snapshotTries reads a light once more before it is left out of the show.
const snapshotTries = 2

// Snapshot reads the state of each light, one at a time within rate frames per second,
// snapshotTries tries of answerWait each. Lights that do not answer are left out.
// It stops early when ctx is done.
func Snapshot(ctx context.Context, c Conn, lights []string, rate float64) map[string]z2m.State {
	snaps := map[string]z2m.State{}
	for _, l := range lights {
		for attempt := 0; attempt < snapshotTries; attempt++ {
			sent := time.Now()
			_ = c.Get(l, query)
			st, ok := c.Await(ctx, l, sent, answerWait)
			if !cli.Sleep(ctx, pace(queryFrames, rate)-time.Since(sent)) {
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
