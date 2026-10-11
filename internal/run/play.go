package run

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run/engine"
)

// Hooks let a caller follow one show. Both are optional.
type Hooks struct {
	Status   func(engine.Stats) // when play starts and every 10 s, from the show loop; must not block
	Stopping func()             // once, right before the restore
}

// Play runs one show to a metronome until ctx is done, then restores every light.
// It returns 0, or 1 when the show could not start, after the third panic, or when
// the broker is gone at stop.
func Play(ctx context.Context, cfg config.Config, bpm float64, h Hooks) int {
	r, err := connect(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { r.link.Close() }()
	r.hooks = h

	limit, code, ok := r.start(ctx)
	if !ok {
		return code
	}
	fmt.Printf("show: %d lights, %g BPM, budget %g frames/s\n", len(r.cfg.Lights()), bpm, r.cfg.Budget.MaxRate)
	reason, code := r.show(ctx, beat.Metronome{BPM: bpm, Lookahead: time.Second}, limit)
	if h.Stopping != nil {
		h.Stopping()
	}
	fmt.Printf("\nstopping: %s (Ctrl-C again to quit without restoring)\n", reason)
	return r.stop(code)
}
