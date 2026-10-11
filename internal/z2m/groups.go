package z2m

import (
	"encoding/json"
	"time"
)

// OnGroups calls fn with the names of the Zigbee2MQTT groups, from the retained
// bridge/groups message and every update after it.
func (b *Bridge) OnGroups(fn func(names []string, at time.Time)) error {
	return b.c.Subscribe(b.base+"/bridge/groups", func(_ string, p []byte, at time.Time) {
		if names, err := groupNames(p); err == nil {
			fn(names, at)
		}
	})
}

func groupNames(p []byte) ([]string, error) {
	var gs []struct {
		Name string `json:"friendly_name"`
	}
	if err := json.Unmarshal(p, &gs); err != nil {
		return nil, err
	}
	names := make([]string, len(gs))
	for i, g := range gs {
		names[i] = g.Name
	}
	return names, nil
}
