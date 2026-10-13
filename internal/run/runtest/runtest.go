// Package runtest holds the fixtures shared by the tests of the run packages.
package runtest

import (
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// T0 is the start of every test clock.
var T0 = time.Date(2026, 10, 11, 21, 0, 0, 0, time.UTC)

// At is T0 plus ms milliseconds.
func At(ms int) time.Time { return T0.Add(time.Duration(ms) * time.Millisecond) }

// Config has lamp-1..3 in one color fixture with mode, lamp-4 breathing, a budget of 100 frames/s at 50 ms,
// no canary, the walk variant and a moment every bar.
func Config(mode config.Mode) config.Config {
	effects := config.DefaultEffects
	effects.Palette = config.DefaultPalette
	effects.Variant = "walk"
	effects.Pace = music.PerBar
	return config.Config{
		MQTT:   config.MQTT{Broker: "b:1883", BaseTopic: "zigbee2mqtt"},
		Budget: config.Budget{MaxRate: 100, LatencyMS: 50},
		Fixtures: []config.Fixture{
			{Name: "pendant", Role: z2m.Color, Mode: mode, Lights: []string{"lamp-1", "lamp-2", "lamp-3"}},
			{Name: "floor", Role: z2m.Breathe, Lights: []string{"lamp-4"}},
		},
		Effects: effects,
	}
}

// Beat is beat i at music.DefaultBPM.
func Beat(i int) music.Beat {
	period := music.Track{BPM: music.DefaultBPM}.Period()
	return music.Beat{At: T0.Add(time.Duration(i) * period), Index: i, InBar: i % music.PerBar, Energy: music.UnknownEnergy}
}
