package show

import (
	"fmt"
	"slices"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

const colorBlend = 0.7

type pace struct {
	beats     int
	rate      float64
	bpm       float64
	stretched int
}

func budgetRate(c config.Config) float64 {
	if c.Canary == "" {
		return c.Budget.MaxRate
	}
	return c.Budget.MaxRate - 1/flow.CanaryEvery.Seconds()
}

func (c pace) period() time.Duration { return music.Track{BPM: c.bpm}.Period() }

func (c pace) now(quiet bool) int {
	i := slices.Index(config.Paces, c.beats)
	if !quiet || i < 0 || i == len(config.Paces)-1 {
		return c.beats
	}
	return config.Paces[i+1]
}

func (c pace) interval(quiet bool, colour float64) int {
	n := c.now(quiet)
	for _, longer := range config.Paces {
		if colour <= c.room(n) {
			break
		}
		n = max(n, longer)
	}
	return n
}

func (c pace) room(n int) float64 {
	return c.rate * (time.Duration(n) * c.period()).Seconds()
}

func (c pace) fall(n int) time.Duration {
	return min(c.period(), time.Duration(n)*c.period()/2)
}

func (c pace) blend(n int) float64 {
	return min(colorBlend, (time.Duration(n) * c.period() / 2).Seconds())
}

func (c *pace) noteStretch(n int, quiet bool) string {
	pace, note := c.now(quiet), ""
	switch {
	case n != pace && n != c.stretched:
		note = fmt.Sprintf("pace every %s; the budget allows every %s at %.0f BPM", music.Span(pace), music.Span(n), c.bpm)
	case n == pace && c.stretched != 0:
		note = fmt.Sprintf("pace every %s again", music.Span(pace))
	}
	c.stretched = 0
	if n != pace {
		c.stretched = n
	}
	return note
}
