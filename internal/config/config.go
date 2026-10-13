// Package config loads and validates the zigbeat configuration file.
package config

import (
	"fmt"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

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

// Limits of budget.audio_offset_ms. Beats arrive music.Lookahead ahead, so a much
// earlier offset would ask for messages before their beat is known; speakers
// that lag more than 2 s are not worth a show.
const MinOffsetMS, MaxOffsetMS = -500, 2000

// OffsetStepMS is the smallest change of the audio offset: about what an ear can tell apart.
const OffsetStepMS = 10

type Budget struct {
	MaxRate       float64 `yaml:"max_rate"`
	LatencyMS     int     `yaml:"latency_ms"`
	AudioOffsetMS int     `yaml:"audio_offset_ms"`
}

// AudioOffset is how much later than the beat the lights should change.
func (b Budget) AudioOffset() time.Duration { return time.Duration(b.AudioOffsetMS) * time.Millisecond }

// CheckOffset checks that the audio offset is within its limits.
func (b Budget) CheckOffset() error {
	if b.AudioOffsetMS < MinOffsetMS || b.AudioOffsetMS > MaxOffsetMS {
		return fmt.Errorf("budget.audio_offset_ms must be %d to %d", MinOffsetMS, MaxOffsetMS)
	}
	return nil
}

type Fixture struct {
	Name   string   `yaml:"name"`
	Role   z2m.Role `yaml:"role"`
	Mode   Mode     `yaml:"mode"`
	Lights []string `yaml:"lights"`
}

type Effects struct {
	Pace           int            `yaml:"pace"` // beats between moments
	Brightness     int            `yaml:"brightness"`
	Palette        []string       `yaml:"palette"`
	BreatheRange   [2]int         `yaml:"breathe_range"`
	BreatheVariant BreatheVariant `yaml:"breathe_variant"` // how the breathe lights move, or rotation
	AlbumPalette   bool           `yaml:"album_palette"`   // the album art's colours when the source has them
	AlbumColors    int            `yaml:"album_colors"`    // colours made from the album art
	Variant        Variant        `yaml:"variant"`         // how the colour lights change, or rotation
	FollowEnergy   bool           `yaml:"follow_energy"`   // quiet parts of the music play calmer
}

// DefaultEffects fills what the effects block leaves out; the palette comes after decoding.
var DefaultEffects = Effects{
	Pace:           2,               // beats: twice the bar of the first show on real lights, which was too slow
	Brightness:     60,              // percent: room for the kick above it
	BreatheRange:   [2]int{10, 100}, // percent: a pulse needs contrast
	BreatheVariant: BreatheRotation, // pulse and flip take turns per 16 bars
	AlbumPalette:   true,            // the colours follow the music when they can
	AlbumColors:    3,               // a few colours keep the mood of the cover
	Variant:        Rotation,        // a new way of changing every 16 bars
	FollowEnergy:   true,            // breakdowns and drops look different; the metronome has no energy
}

// DefaultPalette is used when effects.palette is empty.
var DefaultPalette = []string{"#ff2a6d", "#ff7b00", "#ffd400", "#00e5ff", "#2962ff", "#aa00ff"}

// Lights returns every light in config order.
func (c Config) Lights() []string { return LightsOf(c.Fixtures) }

// LightsOf lists the lights of fixtures in order.
func LightsOf(fixtures []Fixture) []string {
	var out []string
	for _, f := range fixtures {
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
