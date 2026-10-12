// Package run is the light show: `zigbeat run`.
package run

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/engine"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

// drainWait lets every message in flight be confirmed or count as lost, so the
// restore never sends a light a second command.
const drainWait = flow.LostAfter

// Run executes `zigbeat run` and returns an exit code.
func Run(args []string) int {
	o, err := ParseOptions("run", "Runs the light show to the beat source in the config, or a metronome, until Ctrl-C, then restores every light. See the README.", args)
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
	fmt.Println(offsetHelp)
	// Started in the background, reading the terminal would stop the whole show;
	// with SIGTTIN ignored the read fails and calibration is off instead.
	signal.Ignore(syscall.SIGTTIN)
	cal := newCalibration(o.Config, cfg.Budget.AudioOffset())
	offsets := make(chan time.Duration)
	go cal.read(os.Stdin, offsets)
	code := Play(ctx, cfg, Source(cfg, o.BPM), Hooks{Stopping: quitOnInterrupt, Offsets: offsets})
	if hint := cal.unsaved(); hint != "" {
		fmt.Println(hint)
	}
	return code
}

// quitOnInterrupt is the Stopping hook of zigbeat run.
func quitOnInterrupt() string {
	cli.QuitOnInterrupt()
	return cli.QuitHint
}

// show plays until ctx is done or the source ends. A panic ends the show with
// exit code 1; the stack goes to stderr.
func (r *runner) show(ctx context.Context, src music.Source, limit time.Duration) (reason string, code int) {
	r.eng = engine.New(r.cfg, limit, time.Now())
	why, panicked := r.play(ctx, src)
	if !panicked {
		return why, 0
	}
	fmt.Fprintf(os.Stderr, "%s  %s\n", cli.Stamp(), why)
	return "panic; this is a bug, please report it", 1
}

// stop lets the messages in flight land and puts every light back. It returns
// 1 when the broker stayed away, else code.
func (r *runner) stop(code int) int {
	r.drain(drainWait)
	if !r.restore() {
		return 1
	}
	return code
}
