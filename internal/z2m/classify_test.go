package z2m

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func testDevices(t *testing.T) []Device {
	t.Helper()
	data, err := os.ReadFile("testdata/bridge-devices.json")
	if err != nil {
		t.Fatal(err)
	}
	var ds []Device
	if err := json.Unmarshal(data, &ds); err != nil {
		t.Fatal(err)
	}
	return ds
}

func TestClassify(t *testing.T) {
	want := map[string]string{
		"Coordinator":        "the coordinator",
		"bulb":               "breathe",
		"bulb_color_2":       "color",
		"remote":             "battery or end device",
		"0x0017880104e45518": "battery or end device",
		"weather_sensor":     "battery or end device",
		"power_plug":         "switch, never driven",
		"zigfred_plus":       "cover, never driven",
		"white-bulb":         "breathe",
		"hs-bulb":            "breathe",
		"relay":              "switch, never driven",
		"dimmer-module":      "in-wall settings (switch_type): a module",
		"dimming-plug":       `looks like a plug, dimmer or module: "Zigbee dimming smart plug"`,
		"relay-insert":       "several channels (endpoint relay)",
		"tuya-light":         "Tuya datapoint light: transitions and brightness_move are unreliable",
		"disabled-bulb":      "disabled in Zigbee2MQTT",
		"new-bulb":           "interview not finished",
		"old-bulb":           "color",
		"custom-bulb":        "external definition",
	}
	ds := testDevices(t)
	if len(ds) != len(want) {
		t.Fatalf("%d devices in testdata, %d expected", len(ds), len(want))
	}
	for _, d := range ds {
		role, why := Classify(d)
		got := role + why
		if got != want[d.Name] {
			t.Errorf("%s: got %q, want %q", d.Name, got, want[d.Name])
		}
	}
}

func TestClassifyRules(t *testing.T) {
	bulb := func() Device {
		d, _ := Find(testDevices(t), "white-bulb")
		return d
	}
	d := bulb()
	d.Definition.Exposes = append(d.Definition.Exposes, Expose{Type: "numeric", Name: "energy"})
	if _, why := Classify(d); why != "measures power (energy): a plug or module" {
		t.Errorf("metering: %q", why)
	}
	d = bulb()
	d.Definition.Exposes[0].Features[1].Access = 5 // brightness read only
	if _, why := Classify(d); why != "on/off only" {
		t.Errorf("read-only brightness: %q", why)
	}
	d = bulb()
	d.Definition.Exposes = append(d.Definition.Exposes, d.Definition.Exposes[0])
	if _, why := Classify(d); why != "several lights" {
		t.Errorf("two lights: %q", why)
	}
	d = bulb()
	d.Supported = false
	if _, why := Classify(d); why != "not supported by Zigbee2MQTT" {
		t.Errorf("unsupported: %q", why)
	}
	for _, desc := range []string{"Smart dimmer switch", "LED controller module", "Outdoor socket"} {
		d = bulb()
		d.Definition.Description = desc
		if role, _ := Classify(d); role != "" {
			t.Errorf("%q offered as %s", desc, role)
		}
	}
	d = bulb()
	d.Definition.Description = "LED bulb E27, dimmable, warm white"
	if role, why := Classify(d); role != "breathe" {
		t.Errorf("dimmable bulb refused: %s", why)
	}
}

func TestCheck(t *testing.T) {
	ds := testDevices(t)
	cases := []struct{ name, role, err string }{
		{"bulb_color_2", "color", ""},
		{"bulb_color_2", "breathe", ""},
		{"white-bulb", "breathe", ""},
		{"white-bulb", "color", `light "white-bulb" has no xy colour; give it role breathe or remove it`},
		{"power_plug", "breathe", `light "power_plug" is not offered by zigbeat setup (switch, never driven); remove it from the config`},
		{"lamp-9", "color", `light "lamp-9" is not a Zigbee2MQTT device`},
	}
	for _, c := range cases {
		err := Check(ds, c.name, c.role)
		if got := errText(err); got != c.err {
			t.Errorf("%s as %s: got %q, want %q", c.name, c.role, got, c.err)
		}
	}
}

func TestCanaryCandidate(t *testing.T) {
	var got []string
	for _, d := range testDevices(t) {
		if CanaryCandidate(d) {
			got = append(got, d.Name)
		}
	}
	want := "bulb,bulb_color_2,power_plug,zigfred_plus,white-bulb,hs-bulb,relay,dimmer-module,dimming-plug,relay-insert,tuya-light,old-bulb,custom-bulb"
	if strings.Join(got, ",") != want {
		t.Errorf("candidates %v", got)
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
