package main

import (
	"math"
	"math/rand/v2"

	"github.com/siemenvdn/zigbeat/internal/music"
)

const (
	introBars = 4
	dropBars  = 4
	breakBars = 2

	kickHz    = 55
	kickLevel = 0.8
	kickDecay = 0.12
	bassHz    = 110
	bassLevel = 0.25
	padHz     = 80
	padLevel  = 0.05
	hatLevel  = 0.2
	hatDecay  = 0.03

	fullScale = 127
)

type section int

const (
	intro section = iota
	drop
	breakdown
)

func sectionAt(tr track, pickup int, t float64) section {
	period := tr.period().Seconds()
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

func synth(tr track, pickup int, first int64, noise *rand.Rand) []byte {
	period := tr.period().Seconds()
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
