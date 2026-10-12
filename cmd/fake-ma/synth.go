package main

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
)

// The fake's music, per track, in bars from its first downbeat: an intro with
// a low pad and hi-hats, a drop with a kick and a bass, a breakdown that is
// silent, and the drop again until the end.
const (
	introBars = 4
	dropBars  = 4
	breakBars = 2

	kickHz    = 55   // a kick drum's body
	kickLevel = 0.8  // the loudest sound, as in a dance track
	kickDecay = 0.12 // seconds
	bassHz    = 110
	bassLevel = 0.25
	padHz     = 80   // below 150 Hz, so the intro has some low end
	padLevel  = 0.05 // about 15 dB below the drop's low end
	hatLevel  = 0.2  // noise on the offbeats, mostly above 150 Hz
	hatDecay  = 0.03 // seconds

	fullScale = 127 // unsigned 8-bit PCM: silentSample ± 127
)

// section is the part of the track at media time t, in seconds.
type section int

const (
	intro section = iota
	drop
	breakdown
)

// sectionAt finds the section of media time t; the pickup before the first downbeat is intro.
func sectionAt(tr track, pickup int, t float64) section {
	period := time.Duration(float64(time.Minute) / tr.bpm).Seconds()
	firstDown := firstAt.Seconds() + float64(pickup)*period
	bar := int(math.Floor((t - firstDown) / (music.PerBar * period)))
	switch {
	case bar < introBars:
		return intro
	case bar < introBars+dropBars:
		return drop
	case bar < introBars+dropBars+breakBars:
		return breakdown
	}
	return drop
}

// synth makes the waveSamples samples of tr from media sample first on, as MA's tap
// sends them: mono, unsigned 8-bit.
func synth(tr track, pickup int, first int64, noise *rand.Rand) []byte {
	period := time.Duration(float64(time.Minute) / tr.bpm).Seconds()
	out := make([]byte, waveSamples)
	for i := range out {
		t := float64(first+int64(i)) / sampleRate
		x := 0.0
		if since := t - firstAt.Seconds(); since >= 0 {
			beat, off := math.Mod(since, period), math.Mod(since+period/2, period)
			hat := hatLevel * (2*noise.Float64() - 1) * math.Exp(-off/hatDecay)
			switch sectionAt(tr, pickup, t) {
			case intro:
				x = padLevel*math.Sin(2*math.Pi*padHz*t) + hat
			case drop:
				x = kickLevel*math.Sin(2*math.Pi*kickHz*beat)*math.Exp(-beat/kickDecay) +
					bassLevel*math.Sin(2*math.Pi*bassHz*t) + hat
			}
		}
		out[i] = byte(math.RoundToEven(math.Max(-1, math.Min(1, x))*fullScale + silentSample))
	}
	return out
}
