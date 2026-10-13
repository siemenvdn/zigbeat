package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"go.yaml.in/yaml/v3"

	"github.com/siemenvdn/zigbeat/internal/mqtt"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// DefaultPath is the configuration file the commands read without -config.
const DefaultPath = "config.yaml"

// Need says what a command needs from the configuration.
type Need struct {
	Password bool
	Budget   bool
}

// What the commands need.
var (
	ForShow  = Need{Password: true, Budget: true}
	ForDraft = Need{Password: true}
	ForBeats = Need{Budget: true}
)

// Load reads and validates the configuration at path.
func Load(path string, n Need) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Parse(data, os.Getenv, n)
}

// Parse decodes YAML, fills in defaults and validates.
func Parse(data []byte, getenv func(string) string, n Need) (Config, error) {
	if err := removed(data); err != nil {
		return Config{}, err
	}
	c := Config{
		MQTT:    MQTT{Broker: mqtt.DefaultBroker, BaseTopic: z2m.DefaultBaseTopic},
		Effects: DefaultEffects,
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		if errors.Is(err, io.EOF) {
			return Config{}, errors.New("config: the file is empty")
		}
		return Config{}, fmt.Errorf("config: %w", err)
	}
	if len(c.Effects.Palette) == 0 {
		c.Effects.Palette = DefaultPalette
	}
	for i, f := range c.Fixtures {
		if f.Role == z2m.Color && f.Mode == "" {
			c.Fixtures[i].Mode = Alternate
		}
	}
	if c.MQTT.PasswordEnv != "" {
		c.MQTT.Password = getenv(c.MQTT.PasswordEnv)
		if c.MQTT.Password == "" && n.Password {
			return Config{}, fmt.Errorf("config: mqtt.password_env: $%s is not set", c.MQTT.PasswordEnv)
		}
	}
	if m := c.MusicAssistant; m != nil && m.TokenEnv != "" {
		m.Token = getenv(m.TokenEnv)
	}
	if err := c.validate(n.Budget); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return c, nil
}
