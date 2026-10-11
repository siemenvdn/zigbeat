// Package run is the light show: `zigbeat run`.
package run

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run/engine"
)

// Run executes `zigbeat run` and returns an exit code.
func Run(args []string) int {
	o, err := parseFlags(args)
	if code, stop := cli.Exit(err); stop {
		return code
	}
	cfg, err := config.Load(o.config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx, stopSignals := signal.NotifyContext(context.Background(), cli.StopSignals...)
	defer stopSignals()
	return Play(ctx, cfg, o.bpm, Hooks{Stopping: stopSignals})
}

// show plays until ctx is done or the source ends. A panic starts a fresh
// engine; the third one ends the show with exit code 1.
func (r *runner) show(ctx context.Context, src beat.Source, limit time.Duration) (reason string, code int) {
	for panics := 0; ; {
		r.eng = engine.New(r.cfg, limit, time.Now())
		why, panicked := r.play(ctx, src)
		if !panicked {
			return why, 0
		}
		fmt.Fprintf(os.Stderr, "%s  %s\n", stamp(), why)
		if panics++; panics == 3 {
			return "the third panic; this is a bug, please report it", 1
		}
		r.drain(2 * time.Second)
		fmt.Println("restarting the show")
	}
}

// stop lets the messages in flight land and puts every light back. It returns
// 1 when the broker stayed away, else code.
func (r *runner) stop(code int) int {
	r.drain(2 * time.Second)
	if !r.restore() {
		return 1
	}
	return code
}
