// Package show plans the light show: which lights change at each moment, and how.
package show

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Message is one message the planner wants to land at a moment.
type Message struct {
	Light   string
	Payload map[string]any
	Land    time.Time
	Drop    bool   // may be dropped when it cannot leave on time
	before  string // a colour change: the light's colour until it lands
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
	setting      config.Variant // effects.variant
	playing      config.Variant // the variant of the last moment
	rng          *rand.Rand
	variantState // deck, turn and target of the variant that plays
	whiteState   // the breathe variant, levels and turn of the breathe lights
	energyState  // whether the show follows the energy, and plays a quiet part
	moments      int
	pace         int     // effects.pace: beats between moments
	rate         float64 // frames per second the moments may use
	stretched    int     // the interval the budget stretched the pace to; 0 when it did not
	bpm          float64 // the tempo of the current track
	base         int     // Zigbee2MQTT brightness 0..z2m.MaxBrightness
	kick         int
	low, high    int

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
		rate:      budgetRate(c),
	}
	p.SetEffects(c.Effects)
	for _, f := range c.Fixtures {
		if f.Role == z2m.Breathe {
			p.breathe = append(p.breathe, f.Lights...)
			continue
		}
		p.fixtures = append(p.fixtures, f)
		p.nColor += len(f.Lights)
		for _, l := range f.Lights {
			p.fixtureOf[l] = f.Lights
		}
	}
	p.lit = make([]bool, len(p.breathe))
	return p
}

// level converts percent to Zigbee2MQTT brightness.
func level(percent int) int {
	return int(math.Round(float64(percent) * z2m.MaxBrightness / 100))
}

// Moments counts the moments planned so far.
func (p *Planner) Moments() int { return p.moments }

// Beat returns the messages for b, or nothing when b is not a moment: the
// colour changes, their kicks when they fit the room, then the white. calm
// leaves out the kicks and holds the breathe lights; a quiet part plays calm too, one pace slower.
func (p *Planner) Beat(b music.Beat, calm bool) []Message {
	p.lastBeat, p.lastPeriod, p.settled = b.At, p.period(), false
	p.hear(b)
	n := p.interval()
	if b.Index%n != 0 {
		return nil
	}
	p.noteStretch(n)
	p.noteVariants(b)
	calm = calm || p.quiet
	room := p.room(n)
	st := stroke{at: b.At, fall: p.fall(n), blend: p.blend(n)}
	out := p.colour(st, calm, room)
	out = append(out, p.white(st, calm, room-float64(framesOf(out)))...)
	p.moments++
	return out
}
