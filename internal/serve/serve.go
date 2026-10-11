// Package serve runs the show as a service that Home Assistant switches on and off: `zigbeat serve`.
package serve

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run"
)

type options struct {
	config string
	bpm    float64
}

func parseFlags(args []string) (options, error) {
	var o options
	fs := cli.NewFlagSet("serve", "usage: zigbeat serve [-config config.yaml] [-bpm 120]",
		"Waits for the switch in Home Assistant, runs the show to a metronome while it is on and restores every light when it goes off. See the README.")
	fs.StringVar(&o.config, "config", "config.yaml", "configuration file")
	fs.Float64Var(&o.bpm, "bpm", 120, "metronome tempo, 30 to 300")
	if err := cli.Parse(fs, args); err != nil {
		return o, err
	}
	if o.bpm < 30 || o.bpm > 300 {
		return o, fmt.Errorf("-bpm must be 30 to 300")
	}
	return o, nil
}

// Run executes `zigbeat serve` and returns an exit code.
func Run(args []string, version string) int {
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
	// After the first signal a second one quits at once, without restoring.
	go func() { <-ctx.Done(); stopSignals() }()

	l := newLink(cfg, version)
	if err := l.connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	linkCtx, closeLink := context.WithCancel(context.Background())
	linkDone := make(chan struct{})
	go func() { l.run(linkCtx); close(linkDone) }()
	fmt.Printf("zigbeat serve: %d lights, %g BPM, registered in Home Assistant as zigbeat\n", len(cfg.Lights()), o.bpm)

	s := &server{
		show:  func(ctx context.Context, h run.Hooks) int { return run.Play(ctx, cfg, o.bpm, h) },
		cmds:  l.cmds,
		put:   l.put,
		total: len(cfg.Lights()),
	}
	s.loop(ctx)
	closeLink()
	<-linkDone
	return 0
}
