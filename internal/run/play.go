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
	Status   func(engine.Stats)   // when play starts and every statusEvery, from the show loop; must not block
	Stopping func() string        // once, right before the restore; returns a hint for the stop line, or ""
	Offsets  <-chan time.Duration // new audio offsets, typed during the show
}

// Play runs one show to the beats of src until ctx is done, then restores every
// light. It returns 0, or 1 when the show could not start, after maxPanics panics,
// or when the broker is gone at stop.
func Play(ctx context.Context, cfg config.Config, src beat.Source, h Hooks) int {
	r, err := connect(ctx, cfg)
	if err != nil {
		if ctx.Err() != nil {
			fmt.Println(cancelled)
			return 0
		}
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
		if hint := h.Stopping(); hint != "" {
			reason += " (" + hint + ")"
		}
	}
	fmt.Printf("\nstopping: %s\n", reason)
	return r.stop(code)
}
