package show

import "time"

// Start turns every light on at its starting level, landing at now.
func (p *Planner) Start(now time.Time) []Message {
	var out []Message
	for _, f := range p.fixtures {
		for _, l := range f.Lights {
			c := p.colorFor([]string{l}, p.fixtureOf[l])
			p.current[l] = c
			out = append(out, Message{Light: l, Land: now,
				Payload: map[string]any{"state": "ON", "brightness": p.base, "color": map[string]string{"hex": c}}})
		}
	}
	for _, l := range p.breathe {
		out = append(out, Message{Light: l, Land: now, Payload: map[string]any{"state": "ON", "brightness": p.low}})
	}
	return out
}
