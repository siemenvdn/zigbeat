package probe

import (
	"fmt"

	"github.com/siemenvdn/zigbeat/internal/config"
)

// fromConfig takes the MQTT settings, the targets and the canary from the
// config at o.config. Flags in set still win, except the MQTT ones: they
// belong in the config's mqtt block.
func (o *options) fromConfig(set map[string]bool) error {
	for _, f := range []string{"broker", "user", "password", "base"} {
		if set[f] {
			return fmt.Errorf("with -config the MQTT settings come from its mqtt block; leave out -%s", f)
		}
	}
	c, err := config.Load(o.config, config.ForDraft)
	if err != nil {
		return err
	}
	o.budget = c.Budget
	o.broker, o.user, o.password, o.base = c.MQTT.Broker, c.MQTT.Username, c.MQTT.Password, c.MQTT.BaseTopic
	if !set["canary"] {
		o.canary = c.Canary
	}
	if set["targets"] {
		return nil
	}
	var color, breathe []string
	for _, f := range c.Fixtures {
		if f.Role == "color" {
			color = append(color, f.Lights...)
		} else {
			breathe = append(breathe, f.Lights...)
		}
	}
	o.targets = color
	if len(color) == 0 {
		o.targets = breathe
		if !set["mode"] {
			o.mode, o.fallback = "brightness", true
		}
	}
	return nil
}
