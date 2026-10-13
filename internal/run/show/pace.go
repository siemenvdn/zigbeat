package show

import (
	"fmt"
	"slices"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

// colorBlend is the longest a colour change blends, in seconds: see blend.
const colorBlend = 0.7

// pace is when moments fall and how much room each one has.
type pace struct {
	beats     int     // effects.pace: beats between moments
	rate      float64 // frames per second the moments may use
	bpm       float64 // the tempo of the current track
	stretched int     // the interval the budget stretched the pace to; 0 when it did not
}

// budgetRate is the frames per second the moments may use: the budget, less
// the canary's reads.
func budgetRate(c config.Config) float64 {
	if c.Canary == "" {
		return c.Budget.MaxRate
	}
	return c.Budget.MaxRate - 1/flow.CanaryEvery.Seconds()
}

// period is the time between two beats of the current track.
func (c pace) period() time.Duration { return music.Track{BPM: c.bpm}.Period() }

// now is the pace that plays: effects.pace, one step slower in a quiet part.
func (c pace) now(quiet bool) int {
	i := slices.Index(config.Paces, c.beats)
	if !quiet || i < 0 || i == len(config.Paces)-1 {
		return c.beats
	}
	return config.Paces[i+1]
}

// interval is the beats between moments: the pace that plays, or the next longer of
// config.Paces while colour frames, a moment's colour changes without kicks, do
// not fit its room. A beat is a moment when its index is a multiple of it, so
// moments stay on the grid when the pace or the tempo changes, and a pickup beat
// of a new track is a moment only on that track's grid.
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

// room is how many frames a moment n beats long may plan.
func (c pace) room(n int) float64 {
	return c.rate * (time.Duration(n) * c.period()).Seconds()
}

// fall is when a kick's return and a pulse's fall land after their moment:
// half the interval, at most a beat, so before the next moment's kick.
func (c pace) fall(n int) time.Duration {
	return min(c.period(), time.Duration(n)*c.period()/2)
}

// blend is how long a colour change blends: colorBlend, at most half the
// interval. The bulbs of a fixture get their messages one after another, so
// they show the colour together only once the last one has blended.
func (c pace) blend(n int) float64 {
	return min(colorBlend, (time.Duration(n) * c.period() / 2).Seconds())
}

// noteStretch says when the budget stretches the pace to n, and when it no longer does.
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
