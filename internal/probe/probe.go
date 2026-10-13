// Package probe measures how many light commands per second a Zigbee network handles before commands lag or
// the rest of the network slows down.
package probe

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

const (
	countdown = 5 * time.Second

	stepOverhead = 2 * time.Second
)

// Run executes `zigbeat probe` with the given arguments and returns an exit code.
func Run(args []string) int {
	cfg, err := parseFlags(args)
	if code, stop := cli.Exit(err); stop {
		return code
	}
	if cfg.fallback {
		fmt.Printf("no color lights in %s; probing the %d breathe lights in brightness mode\n", cfg.config, len(cfg.targets))
	}
	ctx, stopSignals := cli.StopContext()
	defer stopSignals()
	s, err := connect(ctx, cfg)
	if err != nil {
		return cli.Fail(err)
	}
	defer s.Close()
	if code, ok := s.prepare(); !ok {
		return code
	}
	s.startBackground()
	results, base, touched, err := s.measure()
	if touched && !s.connLost() {
		s.restore()
	}
	s.finish(results, base)
	if (err == nil || err == errInterrupted) && !s.saveBudget(results) {
		return 1
	}
	switch err {
	case nil:
		return 0
	case errInterrupted:
		return cli.ExitInterrupted
	}
	return 1
}

var (
	errInterrupted = errors.New("interrupted")
	errFailed      = errors.New("failed")
)

func (s *session) prepare() (code int, ok bool) {
	if err := s.refuseTargets(); err != nil {
		return cli.Fail(err), false
	}
	if !s.saveState() {
		if s.ctx.Err() != nil {
			return cli.ExitInterrupted, false
		}
		return 1, false
	}
	if !s.canaryAnswers() {
		if s.ctx.Err() != nil {
			fmt.Println("cancelled, nothing was changed")
			return cli.ExitInterrupted, false
		}
		return 1, false
	}
	cfg := s.cfg
	est := cfg.baseline + time.Duration(len(cfg.rates))*(cfg.step+cfg.cooldown+stepOverhead)
	fmt.Printf("zigbeat probe: %d targets, canary %s, mode %s, palette %d\n", len(cfg.targets), cfg.canary, cfg.mode, s.gen.Size())
	fmt.Printf("rates %v cmd/s, then +%g%% per step until the first sign of trouble (at most %d)\n", cfg.rates, climbPercent(), maxRate)
	fmt.Printf("%s per step, %s cooldown, at least %s in total\n", cfg.step, cfg.cooldown, est.Round(10*time.Second))
	fmt.Printf("current state (for manual restore): %s\n", s.snapJSON)
	if !cfg.yes {
		fmt.Printf("The lights will flash. Starting in %s, Ctrl-C to cancel.\n", cli.Seconds(countdown))
		if !s.sleep(countdown) {
			fmt.Println("cancelled, nothing was changed")
			return cli.ExitInterrupted, false
		}
	}
	return 0, true
}

func (s *session) measure() (results []Step, base Stats, touched bool, err error) {
	base, ok := s.baseline()
	switch {
	case !ok && s.connLost():
		return nil, base, false, errFailed
	case !ok:
		s.interrupted()
		return nil, Stats{}, false, errInterrupted
	case base.N < z2m.MinCanaryReads:
		fmt.Fprintln(os.Stderr, "canary answered too rarely during the baseline, aborting")
		return nil, base, false, errFailed
	}
	lim := Limits{CanaryP50: base.P50, CanaryP95: base.P95}
	fmt.Printf("canary at rest: p50 %s ms, p95 %s ms (step fails above %s ms)\n", Millis(base.P50), Millis(base.P95), Millis(lim.CanaryLimit()))
	if !s.selfCheck() {
		return nil, base, true, errFailed
	}
	results = s.climb(lim)
	switch {
	case s.connLost():
		return results, base, true, errFailed
	case s.interrupted():
		return results, base, true, errInterrupted
	}
	return results, base, true, nil
}

func (s *session) interrupted() bool {
	if s.ctx.Err() == nil {
		return false
	}
	s.intOnce.Do(func() {
		cli.QuitOnInterrupt()
		fmt.Printf("\ninterrupted (%s)\n", cli.QuitHint)
	})
	return true
}

func (s *session) connLost() bool {
	if s.Session.Online() {
		return false
	}
	s.lostOnce.Do(func() {
		fmt.Fprintf(os.Stderr, "\nmqtt connection lost: %v\nrestore by hand from: %s\n", s.Err(), s.snapJSON)
	})
	return true
}

func (s *session) finish(results []Step, base Stats) {
	s.stopBg()
	if len(results) > 0 {
		Summary(os.Stdout, results, base, s.cfg.canary)
	}
	samples, logs := s.rec.Snapshot()
	if err := WriteCSV(s.cfg.out, samples, logs); err != nil {
		fmt.Fprintf(os.Stderr, "csv: %v\n", err)
	} else {
		fmt.Printf("raw samples: %s\n", s.cfg.out)
	}
}
