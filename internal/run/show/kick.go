package show

import "github.com/siemenvdn/zigbeat/internal/beat"

const (
	kickPoints = 30  // percentage points above the base brightness
	colorBlend = 0.7 // seconds
	kickFall   = 0.3 // seconds
)

// change gives unit u its next colour, with a brightness kick that falls back
// one beat later; calm leaves out the kick.
func (p *Planner) change(u []string, b beat.Beat, calm bool) []Message {
	p.units++
	c := p.colorFor(u)
	var out []Message
	for _, l := range u {
		// ponytail: a dropped unit still counts as given; the next colour may then repeat once.
		p.current[l] = c
		hex := map[string]string{"hex": c}
		if calm {
			out = append(out, Message{Light: l, Land: b.At, Unit: p.units, Drop: true,
				Payload: map[string]any{"color": hex, "transition": colorBlend}})
			continue
		}
		out = append(out,
			Message{Light: l, Land: b.At, Unit: p.units, Drop: true,
				Payload: map[string]any{"color": hex, "brightness": p.kick, "transition": colorBlend}},
			Message{Light: l, Land: b.At.Add(b.Period()), Unit: p.units,
				Payload: map[string]any{"brightness": p.base, "transition": kickFall}})
	}
	return out
}
