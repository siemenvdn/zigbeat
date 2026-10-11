// Package config loads and validates the zigbeat configuration file.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	MQTT     MQTT      `yaml:"mqtt"`
	Budget   Budget    `yaml:"budget"`
	Canary   string    `yaml:"canary"`
	Fixtures []Fixture `yaml:"fixtures"`
	Effects  Effects   `yaml:"effects"`
}

type MQTT struct {
	Broker      string `yaml:"broker"`
	Username    string `yaml:"username"`
	PasswordEnv string `yaml:"password_env"`
	BaseTopic   string `yaml:"base_topic"`
	Password    string `yaml:"-"` // read from PasswordEnv
}

type Budget struct {
	MaxRate       float64 `yaml:"max_rate"`
	LatencyMS     int     `yaml:"latency_ms"`
	AudioOffsetMS int     `yaml:"audio_offset_ms"`
}

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

// DefaultPalette is used when effects.palette is empty.
var DefaultPalette = []string{"#ff2a6d", "#ff7b00", "#ffd400", "#00e5ff", "#2962ff", "#aa00ff"}

// Load reads and validates the configuration at path.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Parse(data, os.Getenv)
}

// Parse decodes YAML, fills in defaults and validates.
func Parse(data []byte, getenv func(string) string) (Config, error) {
	c := Config{
		MQTT:    MQTT{Broker: "127.0.0.1:1883", BaseTopic: "zigbee2mqtt"},
		Effects: Effects{Cadence: 2 * time.Second, Brightness: 60, BreatheRange: [2]int{30, 80}},
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		if errors.Is(err, io.EOF) {
			return Config{}, errors.New("config: the file is empty")
		}
		return Config{}, fmt.Errorf("config: %v", err)
	}
	if len(c.Effects.Palette) == 0 {
		c.Effects.Palette = DefaultPalette
	}
	for i, f := range c.Fixtures {
		if f.Role == "color" && f.Mode == "" {
			c.Fixtures[i].Mode = "alternate"
		}
	}
	if c.MQTT.PasswordEnv != "" {
		c.MQTT.Password = getenv(c.MQTT.PasswordEnv)
	}
	if err := c.validate(); err != nil {
		return Config{}, fmt.Errorf("config: %v", err)
	}
	return c, nil
}

// Lights returns every light in config order.
func (c Config) Lights() []string {
	var out []string
	for _, f := range c.Fixtures {
		out = append(out, f.Lights...)
	}
	return out
}
