// Package serve runs the show as a service that Home Assistant switches on and off: `zigbeat serve`.
package serve

import (
	"context"
	"fmt"
	"os"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run"
)

// Run executes `zigbeat serve` and returns an exit code.
func Run(args []string, version string) int {
	o, err := run.ParseOptions("serve", "Waits for the switch in Home Assistant, runs the show to a metronome while it is on and restores every light when it goes off. See the README.", args)
	if code, stop := cli.Exit(err); stop {
		return code
	}
	cfg, err := config.Load(o.Config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	ctx, stopSignals := cli.StopContext()
	defer stopSignals()
	// After the first signal a second Ctrl-C quits at once, without restoring.
	go func() { <-ctx.Done(); cli.QuitOnInterrupt() }()

	l := newLink(cfg, version)
	if err := l.connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	linkCtx, closeLink := context.WithCancel(context.Background())
	linkDone := make(chan struct{})
	go func() { l.run(linkCtx); close(linkDone) }()
	fmt.Printf("zigbeat serve: %d lights, %g BPM, registered in Home Assistant as zigbeat\n", len(cfg.Lights()), o.BPM)

	s := &server{
		show:  func(ctx context.Context, h run.Hooks) int { return run.Play(ctx, cfg, o.BPM, h) },
		cmds:  l.cmds,
		put:   l.put,
		total: len(cfg.Lights()),
	}
	s.loop(ctx)
	closeLink()
	<-linkDone
	return 0
}
