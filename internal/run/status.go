package run

import (
	"fmt"
	"time"
)

// status is the line printed every 10 s; it resets the frame counter.
func (e *engine) status(now time.Time) string {
	rate := 0.0
	if secs := now.Sub(e.lastStatus).Seconds(); secs > 0 {
		rate = float64(e.frames) / secs
	}
	e.frames, e.lastStatus = 0, now
	canary := "-"
	if e.canary.last > 0 {
		canary = fmt.Sprintf("%d ms", e.canary.last.Milliseconds())
	}
	return fmt.Sprintf("%s  moments %d  dropped %d  frames/s %.1f  in flight %d  p50 %d ms  backoff %d  canary %s  z2m err %d",
		now.Format("15:04:05"), e.plan.moments, e.dropped, rate, e.inFlight(), e.fl.estimate().Milliseconds(), e.bo.level, canary, e.errs)
}
