package show

import (
	"math/rand/v2"
	"slices"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

const (
	kickPoints = 30  // percentage points above the base brightness
	kickFall   = 0.3 // seconds
	flipEvery  = 4   // moments between alternate fixtures switching separate/together
)

// colorShow is the colour lights' part of the show: which change, to which colour, with a kick or not.
type colorShow struct {
	fixtures  []config.Fixture // color fixtures in config order
	fixtureOf map[string][]string
	n         int // colour lights
	palette   *palette
	current   map[string]string // colour each light was last given
	parts     []part            // the rest of a unit too large for one moment
	cursor    int               // where walk goes on; it outlives a phrase of another variant
	rng       *rand.Rand
	setting   config.Variant // effects.variant
	playing   config.Variant // the variant of the last moment
	variant   variant        // its state; a new one when another variant plays
	base      int            // Zigbee2MQTT brightness 0..z2m.MaxBrightness
	kick      int
}

// part is the rest of a unit too large for one moment, waiting with its colour.
type part struct {
	lights []string
	color  string
}

func newColorShow(fs []config.Fixture, pl *palette) colorShow {
	c := colorShow{fixtureOf: map[string][]string{}, palette: pl, current: map[string]string{},
		rng: rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))}
	for _, f := range fs {
		if f.Role != z2m.Color {
			continue
		}
		c.fixtures = append(c.fixtures, f)
		c.n += len(f.Lights)
		for _, l := range f.Lights {
			c.fixtureOf[l] = f.Lights
		}
	}
	return c
}

func (c *colorShow) style(e config.Effects) {
	c.setting = e.Variant
	c.base = level(e.Brightness)
	c.kick = level(min(e.Brightness+kickPoints, config.MaxPercent))
}

// start turns every colour light on at the base brightness with a first colour.
func (c *colorShow) start(now time.Time) []Message {
	var out []Message
	for _, f := range c.fixtures {
		for _, l := range f.Lights {
			col := c.colorFor([]string{l}, c.fixtureOf[l])
			c.current[l] = col
			out = append(out, Message{Light: l, Land: now,
				Payload: map[string]any{"state": "ON", "brightness": c.base, "color": map[string]string{"hex": col}}})
		}
	}
	return out
}

// play makes v the variant that plays, with a fresh state when it is another one.
func (c *colorShow) play(v config.Variant) {
	if v != c.playing {
		c.playing, c.variant = v, newVariant(v)
	}
}

// plan is the colour lights' part of a moment: the rest of a unit that was too
// large, or the units the variant picks; with kicks when they all fit room.
func (c *colorShow) plan(st stroke, calm bool, room float64) []Message {
	if len(c.parts) > 0 {
		pt := c.parts[0]
		c.parts = c.parts[1:]
		st.kick = !calm && c.fits(len(pt.lights), room)
		return c.paint(pt.lights, pt.color, st) // the variant waits for the rest of a unit
	}
	units := c.variant.pick(c, st)
	st.kick = !calm && c.fits(lightsIn(units), room)
	var out []Message
	for _, u := range units {
		out = append(out, c.change(u, c.variant.color(c, u), st)...)
	}
	return out
}

// change gives unit u colour col. A unit of more than flow.MaxPerMoment lights
// changes in parts, one per moment, all in this colour.
func (c *colorShow) change(u []string, col string, st stroke) []Message {
	ps := split(u)
	for _, rest := range ps[1:] {
		c.parts = append(c.parts, part{rest, col})
	}
	return c.paint(ps[0], col, st)
}

// paint gives lights colour col, with a brightness kick that falls back after
// st.fall when the stroke kicks.
func (c *colorShow) paint(lights []string, col string, st stroke) []Message {
	var out []Message
	for _, l := range lights {
		before := c.current[l]
		c.current[l] = col
		out = append(out, Message{Light: l, Land: st.at, Drop: true, before: before, Payload: c.changePayload(col, st)})
		if st.kick {
			out = append(out, Message{Light: l, Land: st.at.Add(st.fall), Payload: c.returnPayload()})
		}
	}
	return out
}

// changePayload is a colour change, with the kick's brightness when it kicks.
func (c *colorShow) changePayload(col string, st stroke) map[string]any {
	pl := map[string]any{"color": map[string]string{"hex": col}, "transition": st.blend}
	if st.kick {
		pl["brightness"] = c.kick
	}
	return pl
}

// returnPayload takes a kicked light back to the base brightness.
func (c *colorShow) returnPayload() map[string]any {
	return map[string]any{"brightness": c.base, "transition": kickFall}
}

// missed takes back the colours of dropped messages, newest first.
func (c *colorShow) missed(ms []Message) {
	for _, m := range slices.Backward(ms) {
		if col, ok := m.Payload["color"].(map[string]string); ok && c.current[m.Light] == col["hex"] {
			c.current[m.Light] = m.before
		}
	}
}

// colorFor picks the next palette colour that no light in avoid shows, else
// one the unit itself does not show.
func (c *colorShow) colorFor(u, avoid []string) string {
	shown, own := map[string]bool{}, map[string]bool{}
	for _, l := range avoid {
		shown[c.current[l]] = true
	}
	for _, l := range u {
		own[c.current[l]] = true
	}
	if col, ok := c.palette.take(shown); ok {
		return col
	}
	if col, ok := c.palette.take(own); ok {
		return col
	}
	col, _ := c.palette.take(nil)
	return col
}

// unitList lists the units of moment m: one per light, or one per together fixture.
func (c *colorShow) unitList(m int) [][]string {
	together := m/flipEvery%2 == 1
	var us [][]string
	for _, f := range c.fixtures {
		if f.Mode == config.Together || f.Mode == config.Alternate && together {
			us = append(us, f.Lights)
			continue
		}
		for _, l := range f.Lights {
			us = append(us, []string{l})
		}
	}
	return us
}

// shows reports whether every light of u shows colour col.
func (c *colorShow) shows(u []string, col string) bool {
	for _, l := range u {
		if c.current[l] != col {
			return false
		}
	}
	return true
}

// outside lists the colour lights that are not in u.
func (c *colorShow) outside(u []string) []string {
	var out []string
	for _, f := range c.fixtures {
		for _, l := range f.Lights {
			if !slices.Contains(u, l) {
				out = append(out, l)
			}
		}
	}
	return out
}

// fits reports whether lights colour changes with their kicks fit room.
func (c *colorShow) fits(lights int, room float64) bool {
	return float64(lights*c.changeFrames(true)) <= room
}

// minFrames is the least a moment's colour changes cost: without kicks.
func (c *colorShow) minFrames() int { return min(flow.MaxPerMoment, c.n) * c.changeFrames(false) }

// changeFrames is what one light's colour change costs, counted as the rate
// limiter counts it: with its kick, also the return.
func (c *colorShow) changeFrames(kick bool) int {
	n := z2m.Frames(c.changePayload("", stroke{blend: colorBlend, kick: kick}))
	if kick {
		n += z2m.Frames(c.returnPayload())
	}
	return n
}

// lightsIn counts the lights units change at this moment: a unit too large
// changes its first part now.
func lightsIn(units [][]string) int {
	n := 0
	for _, u := range units {
		n += len(split(u)[0])
	}
	return n
}
