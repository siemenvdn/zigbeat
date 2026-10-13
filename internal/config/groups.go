package config

import "fmt"

// RejectGroups refuses a config that names a Zigbee2MQTT group as a light or canary.
func (c Config) RejectGroups(groups []string) error {
	for _, g := range groups {
		for _, l := range append(c.Lights(), c.Canary) {
			if l == g {
				return fmt.Errorf("%q is a Zigbee2MQTT group; list its lights one by one instead", g)
			}
		}
	}
	return nil
}
