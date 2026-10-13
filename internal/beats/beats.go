// Package beats prints the beats from Music Assistant without touching a light: `zigbeat beats`.
package beats

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
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
	cfg, err := config.Load(path, config.ForBeats) // beats never connects to MQTT
	if err != nil {
		return cli.Fail(err)
	}
	if cfg.MusicAssistant == nil {
		fmt.Fprintln(os.Stderr, "zigbeat beats needs a music_assistant block in the config")
		return 1
	}
	ctx, stop := cli.StopContext()
	defer stop()
	src := run.Source(cfg, 0)
	fmt.Printf("zigbeat beats: %v; Ctrl-C to stop\n", src)
	print(ctx, src.Events(ctx), cfg.Budget.AudioOffset(), cfg.Effects, os.Stdout)
	return 0
}

// print writes one line per track when it arrives and one line per beat at the
// moment it should sound, beats arriving ahead, until ctx is done or the source ends.
func print(ctx context.Context, events <-chan music.Event, offset time.Duration, e config.Effects, w io.Writer) {
	var due []music.Beat
	t := time.NewTimer(time.Hour) // stopped at once; any duration works
	t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			switch ev := ev.(type) {
			case music.Clear:
				due = nil
				t.Stop()
			case music.Track:
				fmt.Fprintf(w, "%s  %s\n", time.Now().Format(cli.ClockMS), trackLine(ev, e))
			case music.Beat:
				if due = append(due, ev); len(due) == 1 {
					t.Reset(time.Until(ev.At.Add(offset)))
				}
			}
		case <-t.C:
			b := due[0]
			fmt.Fprintf(w, "%s  bar %d beat %d  energy %.2f\n", b.At.Add(offset).Format(cli.ClockMS), b.Bar(), b.InBar+1, b.Energy)
			if due = due[1:]; len(due) > 0 {
				t.Reset(time.Until(due[0].At.Add(offset)))
			}
		}
	}
}
