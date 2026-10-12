package config

import (
	"errors"

	"go.yaml.in/yaml/v3"
)

// removed refuses effects.cadence, which zigbeat no longer reads, with what to
// set instead, before the strict decode calls it an unknown field.
func removed(data []byte) error {
	var old struct {
		Effects struct {
			Cadence any `yaml:"cadence"`
		} `yaml:"effects"`
	}
	if yaml.Unmarshal(data, &old) == nil && old.Effects.Cadence != nil {
		return errors.New("config: effects.cadence is gone: set effects.pace in beats, such as pace: 2")
	}
	return nil
}
