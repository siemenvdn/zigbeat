package run

import (
	"fmt"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
)

// status is the line printed every 10 s; it resets the frame counter.
func (e *engine) status(now time.Time) string {
	rate := 0.0
	if secs := now.Sub(e.lastStatus).Seconds(); secs > 0 {
		rate = float64(e.frames) / secs
	}
	e.frames, e.lastStatus = 0, now
	canary := "-"
	if e.canary.Last() > 0 {
		canary = fmt.Sprintf("%d ms", e.canary.Last().Milliseconds())
	}
	return fmt.Sprintf("%s  moments %d  dropped %d  frames/s %.1f  in flight %d  p50 %d ms  backoff %d  canary %s  z2m err %d",
		now.Format(cli.Clock), e.plan.moments, e.dropped, rate, e.inFlight(), e.fl.Estimate().Milliseconds(), e.bo.Level(), canary, e.errs)
}
