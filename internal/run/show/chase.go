package show

const (
	maxPerMoment = 3 // messages in one moment
	flipEvery    = 4 // moments between alternate fixtures switching separate/together
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

// pick takes units from the cursor until about half the colour lights are covered,
// at most maxPerMoment messages; the first unit always goes, whatever its size.
func (p *Planner) pick() [][]string {
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
