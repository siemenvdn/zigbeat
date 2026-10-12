package show

import "github.com/siemenvdn/zigbeat/internal/run/flow"

// part is the rest of a unit too large for one moment, waiting with its colour.
type part struct {
	lights []string
	color  string
}

// split cuts u into parts of at most flow.MaxPerMoment lights, in order.
func split(u []string) [][]string {
	var ps [][]string
	for len(u) > flow.MaxPerMoment {
		ps = append(ps, u[:flow.MaxPerMoment])
		u = u[flow.MaxPerMoment:]
	}
	return append(ps, u)
}

// nextPart takes the part that waits longest, if any.
func (p *Planner) nextPart() (part, bool) {
	if len(p.parts) == 0 {
		return part{}, false
	}
	pt := p.parts[0]
	p.parts = p.parts[1:]
	return pt, true
}
