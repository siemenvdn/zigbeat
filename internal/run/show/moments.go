package show

import (
	"fmt"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

// interval is the beats between moments: the pace that plays, or the next longer of
// config.Paces while the colour changes of a moment without kicks do not fit
// its room. A beat is a moment when its index is a multiple of it, so moments
// stay on the grid when the pace or the tempo changes, and a pickup beat of a
// new track is a moment only on that track's grid.
func (p *Planner) interval() int {
	n := p.paceNow()
	colour := float64(min(flow.MaxPerMoment, p.nColor) * p.changeFrames(false))
	for _, longer := range config.Paces {
		if colour <= p.room(n) {
			break
		}
		n = max(n, longer)
	}
	return n
}

// room is how many frames a moment n beats long may plan.
func (p *Planner) room(n int) float64 {
	return p.rate * (time.Duration(n) * p.period()).Seconds()
}

// budgetRate is the frames per second the moments may use: the budget, less
// the canary's reads.
func budgetRate(c config.Config) float64 {
	if c.Canary == "" {
		return c.Budget.MaxRate
	}
	return c.Budget.MaxRate - 1/flow.CanaryEvery.Seconds()
}

// fall is when a kick's return and a pulse's fall land after their moment:
// half the interval, at most a beat, so before the next moment's kick.
func (p *Planner) fall(n int) time.Duration {
	return min(p.period(), time.Duration(n)*p.period()/2)
}

// noteStretch tells the operator when the budget stretches the pace, and when it no longer does.
func (p *Planner) noteStretch(n int) {
	pace := p.paceNow()
	switch {
	case n != pace && n != p.stretched:
		p.notes = append(p.notes, fmt.Sprintf("pace every %s; the budget allows every %s at %.0f BPM", music.Span(pace), music.Span(n), p.bpm))
	case n == pace && p.stretched != 0:
		p.notes = append(p.notes, fmt.Sprintf("pace every %s again", music.Span(pace)))
	}
	p.stretched = 0
	if n != pace {
		p.stretched = n
	}
}
