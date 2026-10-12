package show

import "github.com/siemenvdn/zigbeat/internal/run/flow"

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

// pick takes units from the cursor until 1 in chaseShare colour lights are covered,
// at most flow.MaxPerMoment messages; the first unit always goes, whatever its size.
func (p *Planner) pick() [][]string {
	us := p.unitList()
	if len(us) == 0 {
		return nil
	}
	want := (p.nColor + chaseShare - 1) / chaseShare
	var picked [][]string
	lights := 0
	for range us {
		u := us[p.cursor%len(us)]
		if len(picked) > 0 && (lights >= want || lights+len(u) > flow.MaxPerMoment) {
			break
		}
		picked = append(picked, u)
		lights += len(u)
		p.cursor++
	}
	p.cursor %= len(us)
	return picked
}
