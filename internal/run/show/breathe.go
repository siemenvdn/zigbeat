package show

import (
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// breatheRotation is the order of the breathe variants per phrase, pulse first.
var breatheRotation = []config.BreatheVariant{config.Pulse, config.Flip}

// breatheShow is the breathe lights' part of the show.
type breatheShow struct {
	lights    []string
	setting   config.BreatheVariant // effects.breathe_variant
	playing   config.BreatheVariant // the variant of the last moment; "" without breathe lights
	lit       []bool                // per light: at the top of its range
	next      int                   // the light whose turn comes first
	low, high int                   // Zigbee2MQTT brightness

	lastBeat   time.Time     // the latest beat, a moment or not
	lastPeriod time.Duration // its beat period
	settled    bool          // the lights got their low end since lastBeat
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

// start turns every breathe light on at its low end.
func (b *breatheShow) start(now time.Time) []Message {
	var out []Message
	for _, l := range b.lights {
		out = append(out, Message{Light: l, Land: now, Payload: map[string]any{"state": "ON", "brightness": b.low}})
	}
	return out
}

// at is the breathe variant of the moment at index: the one in the config, or
// in rotation pulse and flip per phrase, pulse first.
func (b *breatheShow) at(index int) config.BreatheVariant {
	switch {
	case len(b.lights) == 0:
		return ""
	case b.setting != config.BreatheRotation:
		return b.setting
	}
	return breatheRotation[max(0, index/phraseBeats)%len(breatheRotation)]
}

// heard notes a beat, a moment or not, for settle.
func (b *breatheShow) heard(at time.Time, period time.Duration) {
	b.lastBeat, b.lastPeriod, b.settled = at, period, false
}

// plan is the breathe lights' part of a moment: the variant that plays, or
// when calm only the low end for a light that is high.
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

// pulse jumps lights to the top of the range and lets them fall back after st.fall.
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

// flip sets light i high when i plus the moment is even, low otherwise:
// counter-phase, with a message only when the level changes.
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

// dim gives a light that is high its low end; it is never dropped.
func (b *breatheShow) dim(st stroke, room float64) []Message {
	var out []Message
	for _, i := range b.turns(func(i int) bool { return b.lit[i] }, z2m.Frames(bright(b.low)), room) {
		b.lit[i] = false
		out = append(out, Message{Light: b.lights[i], Land: st.at, Payload: bright(b.low)})
	}
	return out
}

// turns picks up to flow.MaxPerMoment lights for which want holds, from where
// the last moment stopped, while each costs frames within room.
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

// settle gives every light its low end, once per gap: when the latest beat
// sounded more than a beat ago and no newer one came. A flip or a fall that a
// pause cleared can leave a light high while the music is quiet.
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

// bright is a message that sets a light's brightness, which Zigbee2MQTT confirms.
func bright(level int) map[string]any { return map[string]any{"brightness": level} }
