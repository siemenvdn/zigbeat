// Package ha is the Home Assistant side of zigbeat serve: the MQTT topics,
// the device discovery message and the state payload.
package ha

import (
	"encoding/json"
	"maps"
)

// The MQTT topics of zigbeat serve.
const (
	DiscoveryTopic = "homeassistant/device/zigbeat/config"
	AvailTopic     = "zigbeat/availability"
	StateTopic     = "zigbeat/state"
	CommandTopic   = "zigbeat/show/set"
)

// Discovery is the Home Assistant device discovery message for every entity.
func Discovery(version string) []byte {
	measure := map[string]any{"state_class": "measurement"}
	sensor := func(name, key string, opts map[string]any) map[string]any {
		c := map[string]any{"platform": "sensor", "name": name, "value_template": "{{ value_json." + key + " }}"}
		maps.Copy(c, opts)
		return c
	}
	components := map[string]map[string]any{
		"show": {"platform": "switch", "name": "Show", "command_topic": CommandTopic, "icon": "mdi:music",
			"value_template": "{{ value_json.show }}"},
		"status": sensor("Status", "status", map[string]any{"device_class": "enum", "options": Statuses}),
		"frames": sensor("Frames per second", "frames_per_s", map[string]any{"unit_of_measurement": "frames/s",
			"state_class": "measurement", "suggested_display_precision": 1}),
		"latency": sensor("Latency", "latency_ms", map[string]any{"device_class": "duration", "unit_of_measurement": "ms",
			"state_class": "measurement"}),
		"backoff":     sensor("Backoff", "backoff", measure),
		"lights":      sensor("Lights playing", "lights", measure),
		"lights_away": sensor("Lights left alone", "lights_away", measure),
	}
	for id, c := range components {
		c["unique_id"] = "zigbeat_" + id
	}
	b, _ := json.Marshal(map[string]any{
		"device": map[string]any{"identifiers": []string{"zigbeat"}, "name": "zigbeat", "manufacturer": "zigbeat",
			"model": "light show", "sw_version": version},
		"origin":             map[string]any{"name": "zigbeat", "sw_version": version, "support_url": "https://github.com/siemenvdn/zigbeat"},
		"availability_topic": AvailTopic,
		"state_topic":        StateTopic,
		"components":         components,
	})
	return b
}
