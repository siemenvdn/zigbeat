package show

import (
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

const (
	flipEvery = 4 // moments between alternate fixtures switching separate/together
	// chaseShare: a moment changes about 1 in chaseShare of the colour lights, so
	// the chase moves and the room keeps its colour.
	chaseShare = 2
)

// unitList lists the chase units of this moment: one per light, or one per together fixture.
func (p *Planner) unitList() [][]string {
	together := p.moments/flipEvery%2 == 1
	var us [][]string
	for _, f := range p.fixtures {
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

// fill takes units in order until 1 in chaseShare colour lights are covered,
// at most flow.MaxPerMoment messages; the first unit always goes, whatever its size.
func (p *Planner) fill(units [][]string) [][]string {
	want := (p.nColor + chaseShare - 1) / chaseShare
	var picked [][]string
	lights := 0
	for _, u := range units {
		if len(picked) > 0 && (lights >= want || lights+len(u) > flow.MaxPerMoment) {
			break
		}
		picked = append(picked, u)
		lights += len(u)
	}
	return picked
}

// walk takes units in config order from where the last moment stopped.
func (p *Planner) walk() [][]string {
	us := p.unitList()
	if len(us) == 0 {
		return nil
	}
	from := p.cursor % len(us)
	picked := p.fill(append(us[from:len(us):len(us)], us[:from]...))
	p.cursor = (from + len(picked)) % len(us)
	return picked
}
