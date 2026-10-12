// Package show plans the light show: which lights change at each moment, and how.
package show

import (
	"fmt"
	"math"
	"math/rand/v2"
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
	fixtures     []config.Fixture // color fixtures in config order
	fixtureOf    map[string][]string
	nColor       int
	breathe      []string
	palette      []string    // the colours that play
	fallback     []string    // effects.palette, without album colours
	album        bool        // effects.album_palette
	albumColors  int         // effects.album_colors
	art          []music.RGB // the last track's album colours, as the source sent them
	notes        []string
	pi           int
	current      map[string]string // colour each light was last given
	parts        []part            // the rest of a unit too large for one moment
	cursor       int
	setting      string // effects.variant
	playing      string // the variant of the last moment
	rng          *rand.Rand
	variantState // deck, turn and target of the variant that plays
	moments      int
	units        int
	cadence      time.Duration
	every        int     // beats between moments in force; 0 before the first beat
	bpm          float64 // the tempo of the current track
	base         int     // Zigbee2MQTT brightness 0..z2m.MaxBrightness
	kick         int
	low, high    int
	up           bool

	lastBeat   time.Time     // the latest beat, a moment or not
	lastPeriod time.Duration // its beat period
	settled    bool          // the breathe lights got their low end since lastBeat
}

func NewPlanner(c config.Config) *Planner {
	p := &Planner{
		fixtureOf: map[string][]string{},
		palette:   c.Effects.Palette,
		current:   map[string]string{},
		rng:       rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
		bpm:       music.DefaultBPM,
		low:       level(c.Effects.BreatheRange[0]),
		high:      level(c.Effects.BreatheRange[1]),
		up:        true,
	}
	p.SetEffects(c.Effects)
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

// Moments counts the moments planned so far.
func (p *Planner) Moments() int { return p.moments }

// Beat returns the messages for b, or nothing when b is not a moment.
func (p *Planner) Beat(b music.Beat, backoff int) []Message {
	p.lastBeat, p.lastPeriod, p.settled = b.At, p.period(), false
	n := p.spacing(b.Index, backoff)
	if n == 0 {
		return nil
	}
	if v := p.variantAt(b.Index); v != p.playing {
		p.notes = append(p.notes, fmt.Sprintf("variant %s from bar %d", v, (b.Index-b.InBar)/music.PerBar))
		p.playing, p.variantState = v, variantState{}
	}
	calm := backoff >= flow.CalmLevel
	var out []Message
	if pt, ok := p.nextPart(); ok {
		out = p.paint(pt.lights, pt.color, b, calm) // the chase waits for the rest of a unit
	} else {
		for _, u := range p.pickFor(p.playing) {
			out = append(out, p.change(u, p.colorOf(p.playing, u), b, calm)...)
		}
	}
	out = append(out, p.breath(b, n)...)
	p.moments++
	return out
}
