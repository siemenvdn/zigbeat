package run

import (
	"context"
	"fmt"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/engine"
)

// Hooks let a caller follow and steer one show.
type Hooks struct {
	Status   func(engine.Stats)
	Stopping func() string
	Offsets  <-chan time.Duration
	Effects  <-chan config.Effects
}

// Play runs one show to the beats of src until ctx is done, then restores every light.
func Play(ctx context.Context, cfg config.Config, src music.Source, h Hooks) int {
	r, err := connect(ctx, cfg)
	if err != nil {
		if ctx.Err() != nil {
			fmt.Println(cancelled)
			return 0
		}
		return cli.Fail(err)
	}
	defer func() { r.link.Close() }()
	r.hooks = h

	limit, code, ok := r.start(ctx)
	if !ok {
		return code
	}
	fmt.Printf("show: %d lights, %v, audio offset %d ms, budget %g frames/s\n", len(r.cfg.Lights()), src, cfg.Budget.AudioOffset().Milliseconds(), r.cfg.Budget.MaxRate)
	reason, code := r.show(ctx, src, limit)
	if h.Stopping != nil {
		if hint := h.Stopping(); hint != "" {
			reason += " (" + hint + ")"
		}
	}
	fmt.Printf("\nstopping: %s\n", reason)
	return r.stop(code)
}
