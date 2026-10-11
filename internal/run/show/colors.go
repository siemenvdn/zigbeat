package show

// colorFor picks the next palette colour that no light of the fixture shows,
// else one the unit itself does not show.
func (p *Planner) colorFor(u []string) string {
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

func (p *Planner) take(avoid map[string]bool) (string, bool) {
	for i := range p.palette {
		c := p.palette[(p.pi+i)%len(p.palette)]
		if !avoid[c] {
			p.pi = (p.pi + i + 1) % len(p.palette)
			return c, true
		}
	}
	return "", false
}
