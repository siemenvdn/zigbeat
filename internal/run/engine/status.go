package engine

import (
	"fmt"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

// Stats is the show at one status tick.
type Stats struct {
	At           time.Time
	Moments      int
	Dropped      int
	FramesPerSec float64
	InFlight     int
	P50          time.Duration // latency estimate per frame
	Backoff      int
	Canary       time.Duration // last canary answer, 0 before the first
	Errors       int           // Zigbee2MQTT errors
	Lights       int           // lights in the show minus those left alone
}

// Paused reports the deepest backoff level: nothing goes out but canary reads.
func (s Stats) Paused() bool { return s.Backoff >= flow.PauseLevel }

// Stats reports the show since the last call; it resets the frame counter.
func (e *Engine) Stats(now time.Time) Stats {
	rate := 0.0
	if secs := now.Sub(e.lastStatus).Seconds(); secs > 0 {
		rate = float64(e.frames) / secs
	}
	e.frames, e.lastStatus = 0, now
	return Stats{
		At: now, Moments: e.plan.Moments(), Dropped: e.dropped, FramesPerSec: rate, InFlight: e.InFlight(),
		P50: e.fl.Estimate(), Backoff: e.bo.Level(), Canary: e.canary.Last(), Errors: e.errs,
		Lights: len(e.order) - e.awayCount(),
	}
}

// Line is the status line printed at each status tick.
func (s Stats) Line() string {
	canary := "-"
	if s.Canary > 0 {
		canary = fmt.Sprintf("%d ms", s.Canary.Milliseconds())
	}
	return fmt.Sprintf("%s  moments %d  dropped %d  frames/s %.1f  in flight %d  p50 %d ms  backoff %d  canary %s  z2m err %d",
		s.At.Format(cli.Clock), s.Moments, s.Dropped, s.FramesPerSec, s.InFlight, s.P50.Milliseconds(), s.Backoff, canary, s.Errors)
}
