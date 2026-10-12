// Package runtest holds the fixtures shared by the tests of the run packages.
package runtest

import (
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/config"
)

// T0 is the start of every test clock.
var T0 = time.Date(2026, 10, 11, 21, 0, 0, 0, time.UTC)

// At is T0 plus ms milliseconds.
func At(ms int) time.Time { return T0.Add(time.Duration(ms) * time.Millisecond) }

// Config has lamp-1..3 in one color fixture with mode, lamp-4 breathing,
// a budget of 100 frames/s at 50 ms and no canary.
func Config(mode string) config.Config {
	effects := config.DefaultEffects
	effects.Palette = config.DefaultPalette
	return config.Config{
		MQTT:   config.MQTT{Broker: "b:1883", BaseTopic: "zigbee2mqtt"},
		Budget: config.Budget{MaxRate: 100, LatencyMS: 50},
		Fixtures: []config.Fixture{
			{Name: "pendant", Role: "color", Mode: mode, Lights: []string{"lamp-1", "lamp-2", "lamp-3"}},
			{Name: "floor", Role: "breathe", Lights: []string{"lamp-4"}},
		},
		Effects: effects,
	}
}

// Beat is beat i at beat.DefaultBPM; with the default cadence every bar starts a moment.
func Beat(i int) beat.Beat {
	b := beat.Beat{BPM: beat.DefaultBPM, Index: i, InBar: i % beat.PerBar, Energy: 1}
	b.At = T0.Add(time.Duration(i) * b.Period())
	return b
}
