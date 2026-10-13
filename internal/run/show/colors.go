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
	kickPoints = 30
	kickFall   = 0.3
	flipEvery  = 4
)

type colorShow struct {
	fixtures  []config.Fixture
	fixtureOf map[string][]string
	n         int
	palette   *palette
	current   map[string]string
	parts     []part
	cursor    int
	rng       *rand.Rand
	setting   config.Variant
	playing   config.Variant
	variant   variant
	base      int
	kick      int
}

type part struct {
	lights []string
	color  string
}

func newColorShow(fs []config.Fixture, pl *palette) colorShow {
	c := colorShow{fixtureOf: map[string][]string{}, palette: pl, current: map[string]string{}, variant: walk{},
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

func (c *colorShow) play(v config.Variant) {
	if v != c.playing {
		c.playing, c.variant = v, newVariant(v)
	}
}

func (c *colorShow) plan(st stroke, calm bool, room float64) []Message {
	if len(c.parts) > 0 {
		pt := c.parts[0]
		c.parts = c.parts[1:]
		st.kick = !calm && c.fits(len(pt.lights), room)
		return c.paint(pt.lights, pt.color, st)
	}
	units := c.variant.pick(c, st)
	st.kick = !calm && c.fits(lightsIn(units), room)
	var out []Message
	for _, u := range units {
		out = append(out, c.change(u, c.variant.color(c, u), st)...)
	}
	return out
}

func (c *colorShow) change(u []string, col string, st stroke) []Message {
	ps := split(u)
	for _, rest := range ps[1:] {
		c.parts = append(c.parts, part{rest, col})
	}
	return c.paint(ps[0], col, st)
}

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

func (c *colorShow) changePayload(col string, st stroke) map[string]any {
	pl := map[string]any{"color": map[string]string{"hex": col}, "transition": st.blend}
	if st.kick {
		pl["brightness"] = c.kick
	}
	return pl
}

func (c *colorShow) returnPayload() map[string]any {
	return map[string]any{"brightness": c.base, "transition": kickFall}
}

func (c *colorShow) missed(ms []Message) {
	for _, m := range slices.Backward(ms) {
		if col, ok := m.Payload["color"].(map[string]string); ok && c.current[m.Light] == col["hex"] {
			c.current[m.Light] = m.before
		}
	}
}

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

func (c *colorShow) shows(u []string, col string) bool {
	for _, l := range u {
		if c.current[l] != col {
			return false
		}
	}
	return true
}

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

func (c *colorShow) fits(lights int, room float64) bool {
	return float64(lights*c.changeFrames(true)) <= room
}

func (c *colorShow) minFrames() int { return min(flow.MaxPerMoment, c.n) * c.changeFrames(false) }

func (c *colorShow) changeFrames(kick bool) int {
	n := z2m.Frames(c.changePayload("", stroke{blend: colorBlend, kick: kick}))
	if kick {
		n += z2m.Frames(c.returnPayload())
	}
	return n
}

func lightsIn(units [][]string) int {
	n := 0
	for _, u := range units {
		n += len(split(u)[0])
	}
	return n
}
