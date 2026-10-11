package run

import (
	"fmt"

	"github.com/siemenvdn/zigbeat/internal/cli"
)

type options struct {
	config string
	bpm    float64
}

func parseFlags(args []string) (options, error) {
	var o options
	fs := cli.NewFlagSet("run", "usage: zigbeat run [-config config.yaml] [-bpm 120]",
		"Runs the light show to a metronome until Ctrl-C, then restores every light. See the README.")
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
