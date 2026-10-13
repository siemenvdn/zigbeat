package ha

import (
	"errors"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
)

func paceLabel(beats int) string { return "every " + music.Span(beats) }

func paceLabels() []string {
	ls := make([]string, len(config.Paces))
	for i, n := range config.Paces {
		ls[i] = paceLabel(n)
	}
	return ls
}

func pace(p string) (int, error) {
	for _, n := range config.Paces {
		if paceLabel(n) == p {
			return n, nil
		}
	}
	return 0, errors.New("not a pace")
}
