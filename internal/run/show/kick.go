package show

import "github.com/siemenvdn/zigbeat/internal/music"

const (
	kickPoints = 30  // percentage points above the base brightness
	colorBlend = 0.7 // seconds
	kickFall   = 0.3 // seconds
)

// change gives unit u its next colour. A unit of more than flow.MaxPerMoment
// lights changes in parts, one per moment, all in this colour.
func (p *Planner) change(u []string, b music.Beat, calm bool) []Message {
	c := p.colorFor(u)
	ps := split(u)
	for _, rest := range ps[1:] {
		p.parts = append(p.parts, part{rest, c})
	}
	return p.paint(ps[0], c, b, calm)
}

// paint gives lights colour c, with a brightness kick that falls back one beat
// later; calm leaves out the kick. The lights are one moment unit.
func (p *Planner) paint(lights []string, c string, b music.Beat, calm bool) []Message {
	p.units++
	var out []Message
	for _, l := range lights {
		// A dropped unit still counts as given; the next colour may then repeat once.
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
			Message{Light: l, Land: b.At.Add(p.period()), Unit: p.units,
				Payload: map[string]any{"brightness": p.base, "transition": kickFall}})
	}
	return out
}
