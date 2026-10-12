// Package beats prints the beats from Music Assistant without touching a light: `zigbeat beats`.
package beats

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run"
)

// Run executes `zigbeat beats` and returns an exit code.
func Run(args []string) int {
	var path string
	fs := cli.NewFlagSet("beats", "usage: zigbeat beats [-config "+config.DefaultPath+"]",
		"Prints every beat from Music Assistant when it should sound, without touching a light. See the README.")
	fs.StringVar(&path, "config", config.DefaultPath, "configuration file")
	if code, stop := cli.Exit(cli.Parse(fs, args)); stop {
		return code
	}
	cfg, err := config.LoadWithoutMQTT(path) // beats never connects to MQTT
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if cfg.MusicAssistant == nil {
		fmt.Fprintln(os.Stderr, "zigbeat beats needs a music_assistant block in the config")
		return 1
	}
	ctx, stop := cli.StopContext()
	defer stop()
	src := run.Source(cfg, 0)
	fmt.Printf("zigbeat beats: %v; Ctrl-C to stop\n", src)
	print(ctx, src.Beats(ctx), cfg.Budget.AudioOffset(), os.Stdout)
	return 0
}

// print writes one line per beat at the moment it should sound, beats arriving
// ahead, until ctx is done or the source ends.
func print(ctx context.Context, beats <-chan beat.Beat, offset time.Duration, w io.Writer) {
	var due []beat.Beat
	t := time.NewTimer(time.Hour) // stopped at once; any duration works
	t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case b, ok := <-beats:
			if !ok {
				return
			}
			if due = append(due, b); len(due) == 1 {
				t.Reset(time.Until(b.At.Add(offset)))
			}
		case <-t.C:
			b := due[0]
			fmt.Fprintf(w, "%s  bar %d beat %d  %.1f BPM\n", b.At.Add(offset).Format(cli.ClockMS), (b.Index-b.InBar)/beat.PerBar, b.InBar+1, b.BPM)
			if due = due[1:]; len(due) > 0 {
				t.Reset(time.Until(due[0].At.Add(offset)))
			}
		}
	}
}
