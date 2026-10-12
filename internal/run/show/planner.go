// Package show plans the light show: which lights change at each moment, and how.
package show

import (
	"math"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Message is one message the planner wants to land at a moment.
type Message struct {
	Light   string
	Payload map[string]any
	Land    time.Time
	Unit    int  // > 0: belongs to a moment unit, dropped only as a whole
	Drop    bool // may be dropped when it cannot leave on time
}

// Planner decides per moment which lights change and how.
type Planner struct {
	fixtures  []config.Fixture // color fixtures in config order
	fixtureOf map[string][]string
	nColor    int
	breathe   []string
	palette   []string
	pi        int
	current   map[string]string // colour each light was last given
	cursor    int
	moments   int
	units     int
	cadence   time.Duration
	bpm       float64 // the tempo of the current track
	base      int     // Zigbee2MQTT brightness 0..z2m.MaxBrightness
	kick      int
	low, high int
	up        bool

	lastBeat   time.Time     // the latest beat, a moment or not
	lastPeriod time.Duration // its beat period
	settled    bool          // the breathe lights got their low end since lastBeat
}

func NewPlanner(c config.Config) *Planner {
	p := &Planner{
		fixtureOf: map[string][]string{},
		palette:   c.Effects.Palette,
		current:   map[string]string{},
		cadence:   c.Effects.Cadence,
		bpm:       music.DefaultBPM,
		base:      level(c.Effects.Brightness),
		kick:      level(min(c.Effects.Brightness+kickPoints, config.MaxPercent)),
		low:       level(c.Effects.BreatheRange[0]),
		high:      level(c.Effects.BreatheRange[1]),
		up:        true,
	}
	for _, f := range c.Fixtures {
		if f.Role == "breathe" {
			p.breathe = append(p.breathe, f.Lights...)
			continue
		}
		p.fixtures = append(p.fixtures, f)
		p.nColor += len(f.Lights)
		for _, l := range f.Lights {
			p.fixtureOf[l] = f.Lights
		}
	}
	return p
}

// level converts percent to Zigbee2MQTT brightness.
func level(percent int) int {
	return int(math.Round(float64(percent) * z2m.MaxBrightness / 100))
}

// Track takes the properties of the track that plays now.
func (p *Planner) Track(t music.Track) { p.bpm = t.BPM }

// period is the time between two beats of the current track.
func (p *Planner) period() time.Duration { return music.Track{BPM: p.bpm}.Period() }

// Moments counts the moments planned so far.
func (p *Planner) Moments() int { return p.moments }

// Beat returns the messages for b, or nothing when b is not a moment.
func (p *Planner) Beat(b music.Beat, backoff int) []Message {
	p.lastBeat, p.lastPeriod, p.settled = b.At, p.period(), false
	n := interval(p.bpm, p.cadence)
	if backoff >= flow.SparseLevel {
		n *= 2
	}
	if b.Index%n != 0 {
		return nil
	}
	var out []Message
	for _, u := range p.pick() {
		out = append(out, p.change(u, b, backoff >= flow.CalmLevel)...)
	}
	out = append(out, p.breath(b, n)...)
	p.moments++
	return out
}
