package config

import (
	"strings"
	"testing"
	"time"
)

func env(vals map[string]string) func(string) string {
	return func(k string) string { return vals[k] }
}

func TestExampleLoads(t *testing.T) {
	t.Setenv("ZIGBEAT_MQTT_PASSWORD", "secret")
	c, err := Load("../../config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if c.MQTT.Password != "secret" || c.Canary != "far-lamp" || len(c.Fixtures) != 2 {
		t.Errorf("unexpected config: %+v", c)
	}
	if got := strings.Join(c.Lights(), ","); got != "lamp-1,lamp-2,lamp-3,lamp-4" {
		t.Errorf("Lights() = %s", got)
	}
}

func TestDefaults(t *testing.T) {
	c, err := Parse([]byte(`
budget: {max_rate: 10, latency_ms: 50}
fixtures:
  - {name: pendant, role: color, lights: [a, b]}
  - {name: floor, role: breathe, lights: [c]}
`), env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.MQTT.Broker != "127.0.0.1:1883" || c.MQTT.BaseTopic != "zigbee2mqtt" {
		t.Errorf("mqtt defaults: %+v", c.MQTT)
	}
	if c.Effects.Cadence != 2*time.Second || c.Effects.Brightness != 60 || c.Effects.BreatheRange != [2]int{30, 80} || len(c.Effects.Palette) != 6 {
		t.Errorf("effects defaults: %+v", c.Effects)
	}
	if c.Fixtures[0].Mode != "alternate" || c.Fixtures[1].Mode != "" {
		t.Errorf("mode defaults: %+v", c.Fixtures)
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"unknown key":  "budget: {max_rate: 10, latency_ms: 50, speed: 3}\nfixtures: [{name: p, role: color, lights: [a]}]",
		"empty file":   "",
		"password env": "mqtt: {password_env: NOPE}\nbudget: {max_rate: 10, latency_ms: 50}\nfixtures: [{name: p, role: color, lights: [a]}]",
		"bad duration": "budget: {max_rate: 10, latency_ms: 50}\nfixtures: [{name: p, role: color, lights: [a]}]\neffects: {cadence: soon}",
		"short range":  "budget: {max_rate: 10, latency_ms: 50}\nfixtures: [{name: p, role: color, lights: [a]}]\neffects: {breathe_range: [30]}",
	}
	for name, y := range cases {
		if _, err := Parse([]byte(y), env(nil)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func valid() Config {
	return Config{
		MQTT:   MQTT{Broker: "b:1883", BaseTopic: "zigbee2mqtt"},
		Budget: Budget{MaxRate: 10, LatencyMS: 50},
		Canary: "far",
		Fixtures: []Fixture{
			{Name: "pendant", Role: "color", Mode: "alternate", Lights: []string{"a", "b"}},
			{Name: "floor", Role: "breathe", Lights: []string{"c"}},
		},
		Effects: Effects{Cadence: 2 * time.Second, Brightness: 60, Palette: DefaultPalette, BreatheRange: [2]int{30, 80}},
	}
}

func TestValidate(t *testing.T) {
	if err := valid().validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	cases := map[string]func(*Config){
		"no broker":        func(c *Config) { c.MQTT.Broker = "" },
		"no max_rate":      func(c *Config) { c.Budget.MaxRate = 0 },
		"no latency":       func(c *Config) { c.Budget.LatencyMS = 0 },
		"no fixtures":      func(c *Config) { c.Fixtures = nil },
		"no name":          func(c *Config) { c.Fixtures[0].Name = "" },
		"duplicate name":   func(c *Config) { c.Fixtures[1].Name = "pendant" },
		"bad role":         func(c *Config) { c.Fixtures[0].Role = "strobe" },
		"bad mode":         func(c *Config) { c.Fixtures[0].Mode = "random" },
		"mode on breathe":  func(c *Config) { c.Fixtures[1].Mode = "together" },
		"no lights":        func(c *Config) { c.Fixtures[0].Lights = nil },
		"empty light":      func(c *Config) { c.Fixtures[0].Lights = []string{""} },
		"light twice":      func(c *Config) { c.Fixtures[1].Lights = []string{"a"} },
		"canary is light":  func(c *Config) { c.Canary = "b" },
		"cadence too fast": func(c *Config) { c.Effects.Cadence = 500 * time.Millisecond },
		"brightness 0":     func(c *Config) { c.Effects.Brightness = 0 },
		"brightness 101":   func(c *Config) { c.Effects.Brightness = 101 },
		"range inverted":   func(c *Config) { c.Effects.BreatheRange = [2]int{80, 30} },
		"range above 100":  func(c *Config) { c.Effects.BreatheRange = [2]int{30, 120} },
		"one colour":       func(c *Config) { c.Effects.Palette = []string{"#ff0000"} },
		"bad colour":       func(c *Config) { c.Effects.Palette = []string{"#ff0000", "red"} },
	}
	for name, mutate := range cases {
		c := valid()
		c.Fixtures = append([]Fixture(nil), c.Fixtures...)
		mutate(&c)
		if err := c.validate(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestRejectGroups(t *testing.T) {
	c := valid()
	if err := c.RejectGroups([]string{"living-room"}); err != nil {
		t.Errorf("no light is a group: %v", err)
	}
	if err := c.RejectGroups([]string{"b"}); err == nil || !strings.Contains(err.Error(), `"b"`) {
		t.Errorf("light b is a group: %v", err)
	}
	if err := c.RejectGroups([]string{"far"}); err == nil {
		t.Error("the canary is a group")
	}
}
