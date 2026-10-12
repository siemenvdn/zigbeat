package show

import (
	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

// breatheRotation is the order of the breathe variants per phrase, pulse first.
var breatheRotation = []string{"pulse", "flip"}

// whiteState is what the breathe lights keep between moments.
type whiteState struct {
	breatheSetting string // effects.breathe_variant
	breathing      string // the breathe variant of the last moment; "" without breathe lights
	lit            []bool // per breathe light: at the top of its range
	next           int    // the breathe light whose turn comes first
}

// breatheAt is the breathe variant of the moment at index: the one in the
// config, or in rotation pulse and flip per phrase, pulse first.
func (p *Planner) breatheAt(index int) string {
	switch {
	case len(p.breathe) == 0:
		return ""
	case p.breatheSetting != "rotation":
		return p.breatheSetting
	}
	return breatheRotation[max(0, index/phraseBeats)%len(breatheRotation)]
}

// white plans the breathe lights' part of a moment: the variant that plays, or
// from the calm level only the low end for a light that is high.
func (p *Planner) white(st stroke, calm bool) []Message {
	switch {
	case len(p.breathe) == 0:
		return nil
	case calm:
		return p.dim(st)
	case p.breathing == "flip":
		return p.flip(st)
	}
	return p.pulse(st)
}

// pulse jumps lights to the top of the range and lets them fall back after st.fall.
func (p *Planner) pulse(st stroke) []Message {
	var out []Message
	for _, i := range p.turns(func(int) bool { return true }) {
		l := p.breathe[i]
		p.lit[i] = false
		out = append(out,
			Message{Light: l, Land: st.at, Drop: true, Payload: bright(p.high)},
			Message{Light: l, Land: st.at.Add(st.fall), Payload: bright(p.low)})
	}
	return out
}

// flip sets breathe light i high when i plus the moment count is even, low
// otherwise: counter-phase, with a message only when the level changes.
func (p *Planner) flip(st stroke) []Message {
	high := func(i int) bool { return (i+p.moments)%2 == 0 }
	var out []Message
	for _, i := range p.turns(func(i int) bool { return p.lit[i] != high(i) }) {
		p.lit[i] = high(i)
		lvl := p.low
		if p.lit[i] {
			lvl = p.high
		}
		out = append(out, Message{Light: p.breathe[i], Land: st.at, Drop: true, Payload: bright(lvl)})
	}
	return out
}

// dim gives a breathe light that is high its low end; it is never dropped.
func (p *Planner) dim(st stroke) []Message {
	var out []Message
	for _, i := range p.turns(func(i int) bool { return p.lit[i] }) {
		p.lit[i] = false
		out = append(out, Message{Light: p.breathe[i], Land: st.at, Payload: bright(p.low)})
	}
	return out
}

// turns picks up to flow.MaxPerMoment breathe lights for which want holds,
// from where the last moment stopped, so every light gets its turn.
func (p *Planner) turns(want func(i int) bool) []int {
	var picked []int
	for k := range len(p.breathe) {
		if len(picked) == flow.MaxPerMoment {
			break
		}
		if i := (p.next + k) % len(p.breathe); want(i) {
			picked = append(picked, i)
		}
	}
	if len(picked) > 0 {
		p.next = (picked[len(picked)-1] + 1) % len(p.breathe)
	}
	return picked
}

// bright is a message that sets a light's brightness, which Zigbee2MQTT confirms.
func bright(level int) map[string]any { return map[string]any{"brightness": level} }
