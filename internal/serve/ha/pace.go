package ha

import (
	"errors"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
)

// paceLabel is how Home Assistant shows a pace: every beat, every 2 beats, every bar.
func paceLabel(beats int) string { return "every " + music.Span(beats) }

// paceLabels are the select's options, one per pace.
func paceLabels() []string {
	ls := make([]string, len(config.Paces))
	for i, n := range config.Paces {
		ls[i] = paceLabel(n)
	}
	return ls
}

// pace reads a pace label back to its beats.
func pace(p string) (int, error) {
	for _, n := range config.Paces {
		if paceLabel(n) == p {
			return n, nil
		}
	}
	return 0, errors.New("not a pace")
}
