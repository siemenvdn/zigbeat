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

// rules say what a command needs from the configuration.
type rules struct {
	password bool // the MQTT password must be set
	budget   bool // budget.max_rate and budget.latency_ms must be set
}

// Load reads and validates the configuration at path.
func Load(path string) (Config, error) { return load(path, rules{password: true, budget: true}) }

// LoadWithoutMQTT is Load for a command that never connects to MQTT, such as
// zigbeat beats: the MQTT password may be unset.
func LoadWithoutMQTT(path string) (Config, error) { return load(path, rules{budget: true}) }

// LoadDraft is Load for a configuration that may have no budget yet: zigbeat
// setup writes it with zeros and zigbeat probe fills it in.
func LoadDraft(path string) (Config, error) { return load(path, rules{password: true}) }

func load(path string, r rules) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return parse(data, os.Getenv, r)
}

// Parse decodes YAML, fills in defaults and validates.
func Parse(data []byte, getenv func(string) string) (Config, error) {
	return parse(data, getenv, rules{password: true, budget: true})
}

// ParseDraft is Parse for a configuration that may have no budget yet.
func ParseDraft(data []byte, getenv func(string) string) (Config, error) {
	return parse(data, getenv, rules{password: true})
}

func parse(data []byte, getenv func(string) string, r rules) (Config, error) {
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
		if c.MQTT.Password == "" && r.password {
			return Config{}, fmt.Errorf("config: mqtt.password_env: $%s is not set", c.MQTT.PasswordEnv)
		}
	}
	if m := c.MusicAssistant; m != nil && m.TokenEnv != "" {
		m.Token = getenv(m.TokenEnv)
	}
	if err := c.validate(r.budget); err != nil {
		return Config{}, fmt.Errorf("config: %v", err)
	}
	return c, nil
}
