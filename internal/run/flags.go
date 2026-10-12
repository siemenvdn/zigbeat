package run

import (
	"fmt"

	"github.com/siemenvdn/zigbeat/internal/cli"
)

// Options are the flags of the commands that play the show: run and serve.
type Options struct {
	Config string
	BPM    float64 // 0: the beat source in the config
}

// ParseOptions parses -config and -bpm for the command name; about goes in -h.
func ParseOptions(name, about string, args []string) (Options, error) {
	var o Options
	fs := cli.NewFlagSet(name, fmt.Sprintf("usage: zigbeat %s [-config config.yaml] [-bpm 120]", name), about)
	fs.StringVar(&o.Config, "config", "config.yaml", "configuration file")
	fs.Float64Var(&o.BPM, "bpm", 0, "play to a metronome at this tempo, 30 to 300, instead of the beat source in the config")
	if err := cli.Parse(fs, args); err != nil {
		return o, err
	}
	if o.BPM != 0 && (o.BPM < 30 || o.BPM > 300) {
		return o, fmt.Errorf("-bpm must be 30 to 300")
	}
	return o, nil
}
