package show

import "github.com/siemenvdn/zigbeat/internal/music"

const (
	kickPoints = 30  // percentage points above the base brightness
	colorBlend = 0.7 // seconds
	kickFall   = 0.3 // seconds
)

// change gives unit u colour c. A unit of more than flow.MaxPerMoment lights
// changes in parts, one per moment, all in this colour.
func (p *Planner) change(u []string, c string, b music.Beat, calm bool) []Message {
	ps := split(u)
	for _, rest := range ps[1:] {
		p.parts = append(p.parts, part{rest, c})
	}
	return p.paint(ps[0], c, b, calm)
}

// paint gives lights colour c, with a brightness kick that falls back one beat
// later; calm leaves out the kick.
func (p *Planner) paint(lights []string, c string, b music.Beat, calm bool) []Message {
	var out []Message
	for _, l := range lights {
		before := p.current[l]
		p.current[l] = c
		hex := map[string]string{"hex": c}
		if calm {
			out = append(out, Message{Light: l, Land: b.At, Drop: true, before: before,
				Payload: map[string]any{"color": hex, "transition": colorBlend}})
			continue
		}
		out = append(out,
			Message{Light: l, Land: b.At, Drop: true, before: before,
				Payload: map[string]any{"color": hex, "brightness": p.kick, "transition": colorBlend}},
			Message{Light: l, Land: b.At.Add(p.period()),
				Payload: map[string]any{"brightness": p.base, "transition": kickFall}})
	}
	return out
}

// Missed takes back the colour of a message the engine dropped, while it is
// still the light's latest: the light counts as showing its colour from before.
func (p *Planner) Missed(m Message) {
	if c, ok := m.Payload["color"].(map[string]string); ok && p.current[m.Light] == c["hex"] {
		p.current[m.Light] = m.before
	}
}
