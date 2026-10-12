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

// Load reads and validates the configuration at path.
func Load(path string) (Config, error) { return load(path, true) }

// LoadWithoutMQTT is Load for a command that never connects to MQTT, such as
// zigbeat beats: the MQTT password may be unset.
func LoadWithoutMQTT(path string) (Config, error) { return load(path, false) }

func load(path string, needPassword bool) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return parse(data, os.Getenv, needPassword)
}

// Parse decodes YAML, fills in defaults and validates.
func Parse(data []byte, getenv func(string) string) (Config, error) {
	return parse(data, getenv, true)
}

// parse is Parse; needPassword says whether the MQTT password must be set.
func parse(data []byte, getenv func(string) string, needPassword bool) (Config, error) {
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
		if c.MQTT.Password == "" && needPassword {
			return Config{}, fmt.Errorf("config: mqtt.password_env: $%s is not set", c.MQTT.PasswordEnv)
		}
	}
	if m := c.MusicAssistant; m != nil && m.TokenEnv != "" {
		m.Token = getenv(m.TokenEnv)
	}
	if err := c.validate(); err != nil {
		return Config{}, fmt.Errorf("config: %v", err)
	}
	return c, nil
}
