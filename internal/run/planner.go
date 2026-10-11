package run

import (
	"math"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/config"
)

const (
	maxPerMoment = 3   // messages in one moment, the in-flight limit
	kickPoints   = 30  // percentage points above the base brightness
	flipEvery    = 4   // moments between alternate fixtures switching separate/together
	anchorEvery  = 4   // moments between absolute breathe brightness
	colorBlend   = 0.7 // seconds
	kickFall     = 0.3 // seconds
)

// planned is one message the planner wants to land at a moment.
type planned struct {
	light   string
	payload map[string]any
	land    time.Time
	unit    int  // > 0: belongs to a moment unit, dropped only as a whole
	drop    bool // may be dropped when it cannot leave on time
}

type planner struct {
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
	base      int // Zigbee2MQTT brightness 0..254
	kick      int
	low, high int
	up        bool
}

func newPlanner(c config.Config) *planner {
	p := &planner{
		fixtureOf: map[string][]string{},
		palette:   c.Effects.Palette,
		current:   map[string]string{},
		cadence:   c.Effects.Cadence,
		base:      level(c.Effects.Brightness),
		kick:      level(min(c.Effects.Brightness+kickPoints, 100)),
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
	return int(math.Round(float64(percent) * 254 / 100))
}

// interval is the number of beats between moments: the smallest of 1, 2, 4, 8, 16
// that lasts at least the cadence.
func interval(bpm float64, cadence time.Duration) int {
	period := time.Duration(float64(time.Minute) / bpm)
	for _, n := range []int{1, 2, 4, 8} {
		if time.Duration(n)*period >= cadence {
			return n
		}
	}
	return 16
}

// start turns every light on at its starting level, landing at now.
func (p *planner) start(now time.Time) []planned {
	var out []planned
	for _, f := range p.fixtures {
		for _, l := range f.Lights {
			c := p.colorFor([]string{l})
			p.current[l] = c
			out = append(out, planned{light: l, land: now,
				payload: map[string]any{"state": "ON", "brightness": p.base, "color": map[string]string{"hex": c}}})
		}
	}
	for _, l := range p.breathe {
		out = append(out, planned{light: l, land: now, payload: map[string]any{"state": "ON", "brightness": p.low}})
	}
	return out
}

// beat returns the messages for b, or nothing when b is not a moment.
func (p *planner) beat(b beat.Beat, backoff int) []planned {
	n := interval(b.BPM, p.cadence)
	if backoff >= 2 {
		n *= 2
	}
	if b.Index%n != 0 {
		return nil
	}
	var out []planned
	for _, u := range p.pick() {
		p.units++
		c := p.colorFor(u)
		for _, l := range u {
			// ponytail: a dropped unit still counts as given; the next colour may then repeat once.
			p.current[l] = c
			hex := map[string]string{"hex": c}
			if backoff >= 1 {
				out = append(out, planned{light: l, land: b.At, unit: p.units, drop: true,
					payload: map[string]any{"color": hex, "transition": colorBlend}})
				continue
			}
			out = append(out,
				planned{light: l, land: b.At, unit: p.units, drop: true,
					payload: map[string]any{"color": hex, "brightness": p.kick, "transition": colorBlend}},
				planned{light: l, land: b.At.Add(b.Period()), unit: p.units,
					payload: map[string]any{"brightness": p.base, "transition": kickFall}})
		}
	}
	out = append(out, p.breath(b, n)...)
	p.moments++
	return out
}

// unitList lists the chase units of this moment: one per light, or one per together fixture.
func (p *planner) unitList() [][]string {
	together := p.moments/flipEvery%2 == 1
	var us [][]string
	for _, f := range p.fixtures {
		if f.Mode == "together" || f.Mode == "alternate" && together {
			us = append(us, f.Lights)
			continue
		}
		for _, l := range f.Lights {
			us = append(us, []string{l})
		}
	}
	return us
}

// pick takes units from the cursor until about half the colour lights are covered,
// at most maxPerMoment messages; the first unit always goes, whatever its size.
func (p *planner) pick() [][]string {
	us := p.unitList()
	if len(us) == 0 {
		return nil
	}
	want := (p.nColor + 1) / 2
	var picked [][]string
	lights := 0
	for range us {
		u := us[p.cursor%len(us)]
		if len(picked) > 0 && (lights >= want || lights+len(u) > maxPerMoment) {
			break
		}
		picked = append(picked, u)
		lights += len(u)
		p.cursor++
	}
	p.cursor %= len(us)
	return picked
}

// colorFor picks the next palette colour that no light of the fixture shows,
// else one the unit itself does not show.
func (p *planner) colorFor(u []string) string {
	fixture, own := map[string]bool{}, map[string]bool{}
	for _, l := range p.fixtureOf[u[0]] {
		fixture[p.current[l]] = true
	}
	for _, l := range u {
		own[p.current[l]] = true
	}
	if c, ok := p.take(fixture); ok {
		return c
	}
	if c, ok := p.take(own); ok {
		return c
	}
	c, _ := p.take(nil)
	return c
}

func (p *planner) take(avoid map[string]bool) (string, bool) {
	for i := range p.palette {
		c := p.palette[(p.pi+i)%len(p.palette)]
		if !avoid[c] {
			p.pi = (p.pi + i + 1) % len(p.palette)
			return c, true
		}
	}
	return "", false
}

// breath flips every breathe light; every anchorEvery moments it also sets the
// turning point as an absolute level against drift.
func (p *planner) breath(b beat.Beat, n int) []planned {
	if len(p.breathe) == 0 {
		return nil
	}
	until := time.Duration(n) * b.Period()
	rate := max(1, int(math.Round(float64(p.high-p.low)/until.Seconds())))
	move, bound := rate, p.low
	if !p.up {
		move, bound = -rate, p.high
	}
	p.up = !p.up
	var out []planned
	for _, l := range p.breathe {
		pl := map[string]any{"brightness_move": move}
		if p.moments%anchorEvery == 0 {
			pl["brightness"] = bound
		}
		out = append(out, planned{light: l, land: b.At, payload: pl})
	}
	return out
}
