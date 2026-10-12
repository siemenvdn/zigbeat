package config

import (
	"errors"
	"fmt"
)

// validate checks c; without needBudget a missing budget passes.
func (c Config) validate(needBudget bool) error {
	switch {
	case c.MQTT.Broker == "":
		return errors.New("mqtt.broker is empty")
	case needBudget && c.Budget.MaxRate <= 0:
		return errors.New("budget.max_rate is required: use the budget zigbeat probe suggested")
	case needBudget && c.Budget.LatencyMS <= 0:
		return errors.New("budget.latency_ms is required: the p50 zigbeat probe reported at that load")
	case c.Budget.CheckOffset() != nil:
		return c.Budget.CheckOffset()
	case len(c.Fixtures) == 0:
		return errors.New("fixtures: at least one is required")
	}
	names := map[string]bool{}
	owner := map[string]string{}
	for _, f := range c.Fixtures {
		if f.Name == "" {
			return errors.New("fixtures: every fixture needs a name")
		}
		if names[f.Name] {
			return fmt.Errorf("fixtures: name %q is used twice", f.Name)
		}
		names[f.Name] = true
		switch f.Role {
		case "color":
			if f.Mode != "alternate" && f.Mode != "separate" && f.Mode != "together" {
				return fmt.Errorf("fixture %q: mode must be alternate, separate or together", f.Name)
			}
		case "breathe":
			if f.Mode != "" {
				return fmt.Errorf("fixture %q: mode is only for color fixtures", f.Name)
			}
		default:
			return fmt.Errorf("fixture %q: role must be color or breathe", f.Name)
		}
		if len(f.Lights) == 0 {
			return fmt.Errorf("fixture %q: no lights", f.Name)
		}
		for _, l := range f.Lights {
			if l == "" {
				return fmt.Errorf("fixture %q: empty light name", f.Name)
			}
			if other, ok := owner[l]; ok {
				return fmt.Errorf("light %q is in fixture %q and %q", l, other, f.Name)
			}
			owner[l] = f.Name
		}
	}
	if _, ok := owner[c.Canary]; ok {
		return fmt.Errorf("canary %q must not be one of the lights", c.Canary)
	}
	if c.MusicAssistant != nil {
		if err := c.MusicAssistant.validate(); err != nil {
			return err
		}
	}
	return c.Effects.Validate()
}
