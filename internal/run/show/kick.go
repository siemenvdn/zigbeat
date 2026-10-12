package show

import "time"

const (
	kickPoints = 30  // percentage points above the base brightness
	colorBlend = 0.7 // seconds
	kickFall   = 0.3 // seconds
)

// stroke is how a moment paints: when it lands, when its kicks fall back, and whether it kicks.
type stroke struct {
	at   time.Time
	fall time.Duration
	kick bool
}

// colour plans the colour lights' part of a moment: the rest of a unit that was
// too large, or the units the variant picks.
func (p *Planner) colour(st stroke) []Message {
	if pt, ok := p.nextPart(); ok {
		return p.paint(pt.lights, pt.color, st) // the chase waits for the rest of a unit
	}
	var out []Message
	for _, u := range p.pickFor(p.playing) {
		out = append(out, p.change(u, p.colorOf(p.playing, u), st)...)
	}
	return out
}

// change gives unit u colour c. A unit of more than flow.MaxPerMoment lights
// changes in parts, one per moment, all in this colour.
func (p *Planner) change(u []string, c string, st stroke) []Message {
	ps := split(u)
	for _, rest := range ps[1:] {
		p.parts = append(p.parts, part{rest, c})
	}
	return p.paint(ps[0], c, st)
}

// paint gives lights colour c, with a brightness kick that falls back after
// st.fall when the stroke kicks.
func (p *Planner) paint(lights []string, c string, st stroke) []Message {
	var out []Message
	for _, l := range lights {
		before := p.current[l]
		p.current[l] = c
		out = append(out, Message{Light: l, Land: st.at, Drop: true, before: before, Payload: p.changePayload(c, st.kick)})
		if st.kick {
			out = append(out, Message{Light: l, Land: st.at.Add(st.fall), Payload: p.returnPayload()})
		}
	}
	return out
}

// changePayload is a colour change, with the kick's brightness when it kicks.
func (p *Planner) changePayload(c string, kick bool) map[string]any {
	pl := map[string]any{"color": map[string]string{"hex": c}, "transition": colorBlend}
	if kick {
		pl["brightness"] = p.kick
	}
	return pl
}

// returnPayload takes a kicked light back to the base brightness.
func (p *Planner) returnPayload() map[string]any {
	return map[string]any{"brightness": p.base, "transition": kickFall}
}

// Missed takes back the colour of a message the engine dropped, while it is
// still the light's latest: the light counts as showing its colour from before.
func (p *Planner) Missed(m Message) {
	if c, ok := m.Payload["color"].(map[string]string); ok && p.current[m.Light] == c["hex"] {
		p.current[m.Light] = m.before
	}
}
