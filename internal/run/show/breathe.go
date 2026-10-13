package show

import (
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

var breatheRotation = []config.BreatheVariant{config.Pulse, config.Flip}

type breatheShow struct {
	lights    []string
	setting   config.BreatheVariant
	playing   config.BreatheVariant
	lit       []bool
	next      int
	low, high int

	lastBeat   time.Time
	lastPeriod time.Duration
	settled    bool
}

func newBreatheShow(fs []config.Fixture) breatheShow {
	var b breatheShow
	for _, f := range fs {
		if f.Role == z2m.Breathe {
			b.lights = append(b.lights, f.Lights...)
		}
	}
	b.lit = make([]bool, len(b.lights))
	return b
}

func (b *breatheShow) style(e config.Effects) {
	b.setting = e.BreatheVariant
	b.low, b.high = level(e.BreatheRange[0]), level(e.BreatheRange[1])
}

func (b *breatheShow) start(now time.Time) []Message {
	var out []Message
	for _, l := range b.lights {
		out = append(out, Message{Light: l, Land: now, Payload: map[string]any{"state": "ON", "brightness": b.low}})
	}
	return out
}

func (b *breatheShow) at(index int) config.BreatheVariant {
	switch {
	case len(b.lights) == 0:
		return ""
	case b.setting != config.BreatheRotation:
		return b.setting
	}
	return breatheRotation[max(0, index/phraseBeats)%len(breatheRotation)]
}

func (b *breatheShow) heard(at time.Time, period time.Duration) {
	b.lastBeat, b.lastPeriod, b.settled = at, period, false
}

func (b *breatheShow) plan(st stroke, calm bool, room float64) []Message {
	switch {
	case len(b.lights) == 0:
		return nil
	case calm:
		return b.dim(st, room)
	case b.playing == config.Flip:
		return b.flip(st, room)
	}
	return b.pulse(st, room)
}

func (b *breatheShow) pulse(st stroke, room float64) []Message {
	var out []Message
	for _, i := range b.turns(func(int) bool { return true }, z2m.Frames(bright(b.high))+z2m.Frames(bright(b.low)), room) {
		l := b.lights[i]
		b.lit[i] = false
		out = append(out,
			Message{Light: l, Land: st.at, Drop: true, Payload: bright(b.high)},
			Message{Light: l, Land: st.at.Add(st.fall), Payload: bright(b.low)})
	}
	return out
}

func (b *breatheShow) flip(st stroke, room float64) []Message {
	high := func(i int) bool { return (i+st.moment)%2 == 0 }
	var out []Message
	for _, i := range b.turns(func(i int) bool { return b.lit[i] != high(i) }, z2m.Frames(bright(b.low)), room) {
		b.lit[i] = high(i)
		lvl := b.low
		if b.lit[i] {
			lvl = b.high
		}
		out = append(out, Message{Light: b.lights[i], Land: st.at, Drop: true, Payload: bright(lvl)})
	}
	return out
}

func (b *breatheShow) dim(st stroke, room float64) []Message {
	var out []Message
	for _, i := range b.turns(func(i int) bool { return b.lit[i] }, z2m.Frames(bright(b.low)), room) {
		b.lit[i] = false
		out = append(out, Message{Light: b.lights[i], Land: st.at, Payload: bright(b.low)})
	}
	return out
}

func (b *breatheShow) turns(want func(i int) bool, cost int, room float64) []int {
	var picked []int
	for k := range len(b.lights) {
		if len(picked) == flow.MaxPerMoment || float64((len(picked)+1)*cost) > room {
			break
		}
		if i := (b.next + k) % len(b.lights); want(i) {
			picked = append(picked, i)
		}
	}
	if len(picked) > 0 {
		b.next = (picked[len(picked)-1] + 1) % len(b.lights)
	}
	return picked
}

func (b *breatheShow) settle(now time.Time) []Message {
	if len(b.lights) == 0 || b.settled || b.lastBeat.IsZero() || now.Before(b.lastBeat.Add(b.lastPeriod)) {
		return nil
	}
	b.settled = true
	clear(b.lit)
	var out []Message
	for _, l := range b.lights {
		out = append(out, Message{Light: l, Land: now, Payload: bright(b.low)})
	}
	return out
}

func bright(level int) map[string]any { return map[string]any{"brightness": level} }
