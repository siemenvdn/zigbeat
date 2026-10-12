package run

import (
	"fmt"

	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/cli"
)

// The tempos -bpm takes: the range of music, halved or doubled by a count.
const minBPM, maxBPM = 30, 300

// Options are the flags of the commands that play the show: run and serve.
type Options struct {
	Config string
	BPM    float64 // 0: the beat source in the config
}

// ParseOptions parses -config and -bpm for the command name; about goes in -h.
func ParseOptions(name, about string, args []string) (Options, error) {
	var o Options
	fs := cli.NewFlagSet(name, fmt.Sprintf("usage: zigbeat %s [-config config.yaml] [-bpm %d]", name, beat.DefaultBPM), about)
	fs.StringVar(&o.Config, "config", "config.yaml", "configuration file")
	fs.Float64Var(&o.BPM, "bpm", 0, fmt.Sprintf("play to a metronome at this tempo, %d to %d, instead of the beat source in the config", minBPM, maxBPM))
	if err := cli.Parse(fs, args); err != nil {
		return o, err
	}
	if o.BPM != 0 && (o.BPM < minBPM || o.BPM > maxBPM) {
		return o, fmt.Errorf("-bpm must be %d to %d", minBPM, maxBPM)
	}
	return o, nil
}
