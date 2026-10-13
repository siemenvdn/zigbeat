package main

import (
	"encoding/json"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

func TestDevicesAsZigbeatSeesThem(t *testing.T) {
	names, kindOf, err := parseDevices(defaultDevices + ",ct-1:ct,hs-1:hs,tuya-1:tuya")
	if err != nil {
		t.Fatal(err)
	}
	var ds []z2m.Device
	if err := json.Unmarshal(bridgeDevices(names, kindOf), &ds); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"Coordinator": "", "lamp-1": "color", "lamp-2": "color", "lamp-3": "color", "lamp-4": "breathe", "far-lamp": "color",
		"plug-1": "", "relay-1": "", "dimmer-1": "", "sensor-1": "", "ct-1": "breathe", "hs-1": "breathe", "tuya-1": "",
	}
	if len(ds) != len(want) {
		t.Fatalf("%d devices", len(ds))
	}
	for _, d := range ds {
		if role, why := z2m.Classify(d); string(role) != want[d.Name] {
			t.Errorf("%s: %q (%s), want %q", d.Name, role, why, want[d.Name])
		}
	}
	if _, _, err := parseDevices("lamp-1:disco"); err == nil {
		t.Error("an unknown kind must be refused")
	}
}
