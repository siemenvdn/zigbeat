// Package z2m wraps the Zigbee2MQTT topic layout on top of the mqtt client.
package z2m

import (
	"encoding/json"
	"time"

	"github.com/siemenvdn/zigbeat/internal/mqtt"
)

// DefaultBaseTopic is Zigbee2MQTT's own default base topic.
const DefaultBaseTopic = "zigbee2mqtt"

// LostAfter is how long a command waits for its state before it counts as
// lost: far above a healthy answer of tens of ms, even on a busy mesh.
const LostAfter = 5 * time.Second

// MaxBrightness is Zigbee2MQTT's top brightness; it runs 0..254.
const MaxBrightness = 254

type XY struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type State struct {
	State      string `json:"state,omitempty"`
	Brightness *int   `json:"brightness,omitempty"`
	ColorMode  string `json:"color_mode,omitempty"`
	Color      *XY    `json:"color,omitempty"`
	ColorTemp  *int   `json:"color_temp,omitempty"`
	LinkQual   *int   `json:"linkquality,omitempty"`
}

type LogEntry struct {
	Level     string `json:"level"`
	Message   string `json:"message"`
	Namespace string `json:"namespace"`
}

type Bridge struct {
	c    *mqtt.Client
	base string
}

func New(c *mqtt.Client, base string) *Bridge {
	return &Bridge{c: c, base: base}
}

// StateQuery asks a device for its state: Zigbee2MQTT answers with all of it.
var StateQuery = map[string]any{"state": ""}

func (b *Bridge) Set(device string, payload any) error {
	return b.publish(b.base+"/"+device+"/set", payload)
}

func (b *Bridge) Get(device string, payload any) error {
	return b.publish(b.base+"/"+device+"/get", payload)
}

// OnState delivers every state message Zigbee2MQTT publishes for device from
// now on. A retained one is old and would pass for an answer, so it is skipped.
func (b *Bridge) OnState(device string, fn func(State, time.Time)) error {
	return b.c.SubscribeLive(b.base+"/"+device, func(_ string, p []byte, at time.Time) {
		var s State
		if json.Unmarshal(p, &s) == nil {
			fn(s, at)
		}
	})
}

// OnLog delivers Zigbee2MQTT's own log lines (bridge/logging).
func (b *Bridge) OnLog(fn func(LogEntry, time.Time)) error {
	return b.c.Subscribe(b.base+"/bridge/logging", func(_ string, p []byte, at time.Time) {
		var e LogEntry
		if json.Unmarshal(p, &e) == nil {
			fn(e, at)
		}
	})
}

func (b *Bridge) publish(topic string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return b.c.Publish(topic, data)
}
