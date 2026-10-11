// Package probe measures how many light commands per second a Zigbee network
// handles before commands lag or the rest of the network slows down.
package probe

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
)

// Run executes `zigbeat probe` with the given arguments and returns an exit code.
func Run(args []string) int {
	cfg, err := parseFlags(args)
	switch {
	case err == flag.ErrHelp:
		return 0
	case err == errUsage:
		return 2
	case err != nil:
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	ctx, stopSignals := signal.NotifyContext(context.Background(), cli.StopSignals...)
	defer stopSignals()
	s, err := connect(ctx, cfg, stopSignals)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer s.client.Close()
	if !s.saveState() || !s.canaryAnswers() {
		return 1
	}

	est := cfg.baseline + time.Duration(len(cfg.rates))*(cfg.step+cfg.cooldown+2*time.Second)
	fmt.Printf("zigbeat probe: %d targets, canary %s, mode %s, palette %d\n", len(cfg.targets), cfg.canary, cfg.mode, s.gen.paletteSize())
	fmt.Printf("rates %v cmd/s, then +25%% per step until the first sign of trouble (at most %d)\n", cfg.rates, maxRate)
	fmt.Printf("%s per step, %s cooldown, at least %s in total\n", cfg.step, cfg.cooldown, est.Round(10*time.Second))
	fmt.Printf("current state (for manual restore): %s\n", s.snapJSON)
	if !cfg.yes {
		fmt.Println("The lights will flash. Starting in 5 s, Ctrl-C to cancel.")
		if !s.sleep(5 * time.Second) {
			fmt.Println("cancelled, nothing was changed")
			return 130
		}
	}

	s.startBackground()
	defer s.stopBg()

	base, ok := s.baseline()
	if !ok {
		if s.connLost() {
			return 1
		}
		s.interrupted()
		s.finish(nil, latencyStats{})
		return 130
	}
	if base.n < 3 {
		fmt.Fprintln(os.Stderr, "canary answered too rarely during the baseline, aborting")
		s.finish(nil, base)
		return 1
	}
	lim := limits{canaryBaseP50: base.p50, canaryBaseP95: base.p95}
	fmt.Printf("canary at rest: p50 %s ms, p95 %s ms (step fails above %s ms)\n", ms(base.p50), ms(base.p95), ms(lim.canaryLimit()))

	if !s.selfCheck() {
		s.restore()
		s.finish(nil, base)
		return 1
	}

	results := s.climb(lim)
	if s.connLost() {
		s.finish(results, base)
		return 1
	}
	wasInterrupted := s.interrupted()
	s.restore()
	s.finish(results, base)
	if wasInterrupted {
		return 130
	}
	return 0
}
