// Package run is the light show: `zigbeat run`.
package run

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
)

type options struct {
	config string
	bpm    float64
}

// errUsage means the flag package already printed the problem and the usage.
var errUsage = errors.New("usage")

func parseFlags(args []string) (options, error) {
	var o options
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: zigbeat run [-config config.yaml] [-bpm 120]")
		fmt.Fprintln(fs.Output(), "\nRuns the light show to a metronome until Ctrl-C, then restores every light. See the README.")
		fmt.Fprintln(fs.Output())
		fs.PrintDefaults()
	}
	fs.StringVar(&o.config, "config", "config.yaml", "configuration file")
	fs.Float64Var(&o.bpm, "bpm", 120, "metronome tempo, 30 to 300")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return o, err
		}
		return o, errUsage
	}
	if o.bpm < 30 || o.bpm > 300 {
		return o, fmt.Errorf("-bpm must be 30 to 300")
	}
	return o, nil
}

// Run executes `zigbeat run` and returns an exit code.
func Run(args []string) int {
	o, err := parseFlags(args)
	switch {
	case err == flag.ErrHelp:
		return 0
	case err == errUsage:
		return 2
	case err != nil:
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	cfg, err := config.Load(o.config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	ctx, stopSignals := signal.NotifyContext(context.Background(), cli.StopSignals...)
	defer stopSignals()
	r, err := connect(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { r.link.Close() }()
	if err := r.refuseGroups(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	ok := r.snapshot(ctx)
	limit := time.Duration(0)
	if ok {
		fmt.Printf("current state (for manual restore): %s\n", r.snapJSON)
		limit = r.canaryBaseline(ctx)
	}
	if ctx.Err() != nil {
		fmt.Println("cancelled before the show started, nothing was changed")
		return 0
	}
	if !ok {
		return 1
	}
	fmt.Printf("zigbeat run: %d lights, %g BPM, budget %g frames/s\n", len(r.cfg.Lights()), o.bpm, r.cfg.Budget.MaxRate)

	src := beat.Metronome{BPM: o.bpm, Lookahead: time.Second}
	reason, code := "", 0
	for panics := 0; ; {
		r.eng = newEngine(r.cfg, limit, time.Now())
		why, panicked := r.play(ctx, src)
		reason = why
		if !panicked {
			break
		}
		fmt.Fprintf(os.Stderr, "%s  %s\n", stamp(), why)
		if panics++; panics == 3 {
			reason, code = "the third panic; this is a bug, please report it", 1
			break
		}
		r.drain(2 * time.Second)
		fmt.Println("restarting the show")
	}
	stopSignals()
	fmt.Printf("\nstopping: %s (Ctrl-C again to quit without restoring)\n", reason)
	r.drain(2 * time.Second)
	if !r.restore() {
		return 1
	}
	return code
}
