package setup

import (
	"os"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
)

func TestYAMLName(t *testing.T) {
	for in, want := range map[string]string{
		"pendant-1": "pendant-1", "Living Room 2": "Living Room 2", "lamp, big": `"lamp, big"`, "yes": `"yes"`,
		"0x0017880104e45518": `"0x0017880104e45518"`, "#1": `"#1"`, "keuken/plafond": "keuken/plafond", "[x]": `"[x]"`,
	} {
		if got := yamlName(in); got != want {
			t.Errorf("yamlName(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestNewConfig(t *testing.T) {
	fs := []config.Fixture{
		{Name: "pendant", Role: "color", Lights: []string{"pendant-1", "pendant-2", "pendant-3"}},
		{Name: "side", Role: "color", Mode: "together", Lights: []string{"lamp, big"}},
		{Name: "floor-lamp", Role: "breathe", Lights: []string{"floor-lamp"}},
	}
	m := config.MQTT{Broker: "127.0.0.1:1883", Username: "zigbeat", PasswordEnv: "ZIGBEAT_MQTT_PASSWORD", BaseTopic: "zigbee2mqtt"}
	got := newConfig("config.yaml", m, "hallway-ceiling", fs, time.Date(2026, 10, 12, 21, 0, 0, 0, time.UTC))
	want := `# zigbeat configuration, written by zigbeat setup on 2026-10-12.
# Lights not listed under fixtures are never touched.

mqtt:
  broker: 127.0.0.1:1883
  username: zigbeat
  password_env: ZIGBEAT_MQTT_PASSWORD
  base_topic: zigbee2mqtt

budget:
  max_rate: 0          # frames per second: zigbeat probe -config config.yaml fills it in
  latency_ms: 0        # p50 per frame at that load: zigbeat probe fills it in
  audio_offset_ms: 0   # positive when the speakers play later; type save while calibrating in zigbeat run

canary: hallway-ceiling  # read only, never changed

fixtures:
  - name: pendant
    role: color
    lights: [pendant-1, pendant-2, pendant-3]
  - name: side
    role: color
    mode: together
    lights: ["lamp, big"]
  - name: floor-lamp
    role: breathe
    lights: [floor-lamp]

# effects:
#   cadence: 2s
#   brightness: 60
#   breathe_range: [30, 80]

# music_assistant:
#   url: ws://music-assistant:8095
#   token_env: ZIGBEAT_MA_TOKEN
`
	if got != want {
		t.Errorf("got\n%s", got)
	}
	c, err := config.ParseDraft([]byte(got), func(string) string { return "secret" })
	if err != nil || c.Canary != "hallway-ceiling" || c.Fixtures[1].Lights[0] != "lamp, big" || c.Fixtures[0].Mode != "alternate" {
		t.Errorf("parsed back: %+v, %v", c, err)
	}
	if _, err := config.Parse([]byte(got), os.Getenv); err == nil {
		t.Error("run must refuse a config without a budget")
	}
}
