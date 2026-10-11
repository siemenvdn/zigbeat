package config

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (c Config) validate() error {
	switch {
	case c.MQTT.Broker == "":
		return errors.New("mqtt.broker is empty")
	case c.MQTT.PasswordEnv != "" && c.MQTT.Password == "":
		return fmt.Errorf("mqtt.password_env: $%s is not set", c.MQTT.PasswordEnv)
	case c.Budget.MaxRate <= 0:
		return errors.New("budget.max_rate is required: use the budget zigbeat probe suggested")
	case c.Budget.LatencyMS <= 0:
		return errors.New("budget.latency_ms is required: the p50 zigbeat probe reported at that load")
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
	e := c.Effects
	switch {
	case e.Cadence < time.Second:
		return errors.New("effects.cadence must be at least 1s")
	case e.Brightness < 1 || e.Brightness > 100:
		return errors.New("effects.brightness must be 1 to 100")
	case e.BreatheRange[0] < 1 || e.BreatheRange[1] > 100 || e.BreatheRange[0] >= e.BreatheRange[1]:
		return errors.New("effects.breathe_range must be [low, high] within 1 to 100 with low below high")
	case len(e.Palette) < 2:
		return errors.New("effects.palette needs at least 2 colours")
	}
	for _, p := range e.Palette {
		if !hexColor.MatchString(p) {
			return fmt.Errorf("effects.palette: %q is not a #rrggbb colour", p)
		}
	}
	return nil
}
