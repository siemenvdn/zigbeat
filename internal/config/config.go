// Package config loads and validates the zigbeat configuration file.
package config

import "time"

type Config struct {
	MQTT           MQTT            `yaml:"mqtt"`
	Budget         Budget          `yaml:"budget"`
	Canary         string          `yaml:"canary"`
	Fixtures       []Fixture       `yaml:"fixtures"`
	Effects        Effects         `yaml:"effects"`
	MusicAssistant *MusicAssistant `yaml:"music_assistant"` // nil: the show runs to a metronome
}

type MQTT struct {
	Broker      string `yaml:"broker"`
	Username    string `yaml:"username"`
	PasswordEnv string `yaml:"password_env"`
	BaseTopic   string `yaml:"base_topic"`
	Password    string `yaml:"-"` // read from PasswordEnv
}

// Limits of budget.audio_offset_ms. Beats arrive beat.Lookahead ahead, so a much
// earlier offset would ask for messages before their beat is known; speakers
// that lag more than 2 s are not worth a show.
const MinOffsetMS, MaxOffsetMS = -500, 2000

type Budget struct {
	MaxRate       float64 `yaml:"max_rate"`
	LatencyMS     int     `yaml:"latency_ms"`
	AudioOffsetMS int     `yaml:"audio_offset_ms"`
}

// AudioOffset is how much later than the beat the lights should change.
func (b Budget) AudioOffset() time.Duration { return time.Duration(b.AudioOffsetMS) * time.Millisecond }

type Fixture struct {
	Name   string   `yaml:"name"`
	Role   string   `yaml:"role"`
	Mode   string   `yaml:"mode"`
	Lights []string `yaml:"lights"`
}

type Effects struct {
	Cadence      time.Duration `yaml:"cadence"`
	Brightness   int           `yaml:"brightness"`
	Palette      []string      `yaml:"palette"`
	BreatheRange [2]int        `yaml:"breathe_range"`
}

// DefaultEffects fills what the effects block leaves out; the palette comes after decoding.
var DefaultEffects = Effects{
	Cadence:      2 * time.Second, // a moment once a bar at 120 BPM
	Brightness:   60,              // percent: room for the kick above it
	BreatheRange: [2]int{30, 80},  // percent: a clear swell, never dark or full
}

// DefaultPalette is used when effects.palette is empty.
var DefaultPalette = []string{"#ff2a6d", "#ff7b00", "#ffd400", "#00e5ff", "#2962ff", "#aa00ff"}

// Lights returns every light in config order.
func (c Config) Lights() []string {
	var out []string
	for _, f := range c.Fixtures {
		out = append(out, f.Lights...)
	}
	return out
}

// Keep returns c with only the lights for which keep is true; fixtures left empty are dropped.
func (c Config) Keep(keep func(light string) bool) Config {
	var fs []Fixture
	for _, f := range c.Fixtures {
		var ls []string
		for _, l := range f.Lights {
			if keep(l) {
				ls = append(ls, l)
			}
		}
		if len(ls) > 0 {
			f.Lights = ls
			fs = append(fs, f)
		}
	}
	c.Fixtures = fs
	return c
}
