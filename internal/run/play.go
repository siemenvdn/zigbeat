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

// Hooks let a caller follow and steer one show. All are optional.
type Hooks struct {
	Status   func(engine.Stats)   // when play starts and every 10 s, from the show loop; must not block
	Stopping func()               // once, right before the restore
	Offsets  <-chan time.Duration // new audio offsets, typed during the show
}

// Play runs one show to the beats of src until ctx is done, then restores every
// light. It returns 0, or 1 when the show could not start, after the third panic,
// or when the broker is gone at stop.
func Play(ctx context.Context, cfg config.Config, src beat.Source, h Hooks) int {
	r, err := connect(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { r.link.Close() }()
	r.hooks = h
	r.offset = cfg.Budget.AudioOffset()

	limit, code, ok := r.start(ctx)
	if !ok {
		return code
	}
	fmt.Printf("show: %d lights, %v, audio offset %d ms, budget %g frames/s\n", len(r.cfg.Lights()), src, r.offset.Milliseconds(), r.cfg.Budget.MaxRate)
	reason, code := r.show(ctx, src, limit)
	if h.Stopping != nil {
		h.Stopping()
	}
	fmt.Printf("\nstopping: %s (Ctrl-C again to quit without restoring)\n", reason)
	if r.offset != cfg.Budget.AudioOffset() {
		ms := r.offset.Milliseconds()
		fmt.Printf("audio offset is %d ms; put audio_offset_ms: %d in the config to keep it\n", ms, ms)
	}
	return r.stop(code)
}
